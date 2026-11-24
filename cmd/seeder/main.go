package main

import (
	"bufio"
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"be_region_indonesia/config"
	"be_region_indonesia/pkg/database"
	"github.com/lib/pq"
)

type SeedItem struct {
	Code  string
	Name  string
	Level int16
	Type  string
}

func main() {
	start := time.Now()
	log.Println("[SEEDER] Memulai proses seeding data wilayah Indonesia...")

	cfg := config.MustLoadConfig(".env")
	pool, err := database.NewPostgresConnection(cfg)
	if err != nil {
		log.Fatalf("[FATAL] Gagal menghubungkan ke database: %v", err)
	}
	defer pool.Close()

	sqlPath := filepath.Join("scripts", "data", "wilayah.sql")
	if _, err := os.Stat(sqlPath); os.IsNotExist(err) {
		log.Fatalf("[FATAL] Berkas dataset %s tidak ditemukan", sqlPath)
	}

	items, err := parseWilayahSQL(sqlPath)
	if err != nil {
		log.Fatalf("[FATAL] Gagal parsing wilayah.sql: %v", err)
	}
	log.Printf("[SEEDER] Berhasil mem-parse %d entitas wilayah administratif (Level 1-4).", len(items))

	// Truncate tabel regions sebelum bulk copy
	ctx := context.Background()
	_, err = pool.DB.ExecContext(ctx, "TRUNCATE TABLE regions RESTART IDENTITY;")
	if err != nil {
		log.Fatalf("[FATAL] Gagal mengosongkan tabel regions: %v", err)
	}

	// Bulk insert menggunakan PostgreSQL COPY protocol (pq.CopyIn)
	err = bulkInsert(ctx, pool.DB, items)
	if err != nil {
		log.Fatalf("[FATAL] Gagal bulk insert: %v", err)
	}

	elapsed := time.Since(start)
	log.Printf("[SUCCESS] Seeding selesai! Sebanyak %d data wilayah berhasil dimasukkan dalam tempo %v.", len(items), elapsed)
}

func parseWilayahSQL(path string) ([]SeedItem, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Regex mencocokkan pola: ('11','Aceh') atau ('11.01','Kabupaten Aceh Selatan')
	re := regexp.MustCompile(`^\('([^']+)',\s*'([^']+)'\)`)

	scanner := bufio.NewScanner(file)
	// Naikkan kapasitas buffer untuk baris panjang
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024)

	var items []SeedItem
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "('") {
			continue
		}

		matches := re.FindStringSubmatch(line)
		if len(matches) == 3 {
			code := strings.TrimSpace(matches[1])
			name := strings.TrimSpace(matches[2])

			level, regType := determineLevelAndType(code, name)
			items = append(items, SeedItem{
				Code:  code,
				Name:  strings.ToUpper(name),
				Level: level,
				Type:  regType,
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func determineLevelAndType(code, name string) (int16, string) {
	dots := strings.Count(code, ".")
	upperName := strings.ToUpper(name)

	switch dots {
	case 0:
		return 1, "PROVINSI"
	case 1:
		if strings.HasPrefix(upperName, "KOTA") {
			return 2, "KOTA"
		}
		return 2, "KABUPATEN"
	case 2:
		return 3, "KECAMATAN"
	case 3:
		if strings.HasPrefix(upperName, "DESA") {
			return 4, "DESA"
		}
		return 4, "KELURAHAN"
	default:
		return 5, "KODE_POS"
	}
}

func bulkInsert(ctx context.Context, db *sql.DB, items []SeedItem) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(pq.CopyIn("regions", "code", "name", "level", "type", "created_at", "updated_at"))
	if err != nil {
		return fmt.Errorf("failed to prepare CopyIn statement: %w", err)
	}
	defer stmt.Close()

	now := time.Now()
	for _, item := range items {
		_, err = stmt.Exec(item.Code, item.Name, item.Level, item.Type, now, now)
		if err != nil {
			return fmt.Errorf("failed to exec copy item (code: %s): %w", item.Code, err)
		}
	}

	// Flush data pada pipeline COPY
	_, err = stmt.Exec()
	if err != nil {
		return fmt.Errorf("failed to flush CopyIn: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
