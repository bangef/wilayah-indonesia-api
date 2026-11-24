package main

import (
	"bufio"
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"be_region_indonesia/config"
	"be_region_indonesia/internal/domain"
	"be_region_indonesia/internal/repository/cache"
	"be_region_indonesia/internal/repository/postgres"
	"be_region_indonesia/pkg/database"
)

type ImportStats struct {
	TotalRead     int
	ValidRows     int
	InvalidFormat int
	UpdatedRows   int64
	SkippedRows   int
	Duration      time.Duration
}

type PostalItem struct {
	Code       string
	PostalCode string
}

var postalCodeRegex = regexp.MustCompile(`^[0-9]{5}$`)

func main() {
	filePath := flag.String("file", "data/postal_codes_template.csv", "Path ke berkas CSV sumber kode pos")
	batchSize := flag.Int("batch-size", 1000, "Ukuran chunk batch untuk eksekusi update ke database")
	dryRun := flag.Bool("dry-run", false, "Simulasi eksekusi tanpa mengubah database atau invalidasi cache")
	envPath := flag.String("env", ".env", "Path ke berkas environment .env")
	flag.Parse()

	log.Println("============================================================")
	log.Println("              CSV POSTAL CODE IMPORTER CLI                 ")
	log.Println("============================================================")
	log.Printf("[INFO] Memulai proses import dari: %s\n", *filePath)
	if *dryRun {
		log.Println("[INFO] Mode DRY-RUN AKTIF. Tidak ada data yang akan diubah.")
	}

	start := time.Now()

	// 1. Muat Konfigurasi
	cfg, err := config.LoadConfig(*envPath)
	if err != nil {
		log.Fatalf("[FATAL] Gagal memuat konfigurasi environment: %v", err)
	}

	// 2. Koneksi Database & Redis
	pgPool, err := database.NewPostgresConnection(cfg)
	if err != nil {
		log.Fatalf("[FATAL] Gagal menghubungkan ke database: %v", err)
	}
	defer pgPool.Close()

	redisPool, err := database.NewRedisConnection(cfg)
	if err != nil {
		log.Fatalf("[FATAL] Gagal menghubungkan ke Redis: %v", err)
	}
	defer redisPool.Close()

	repo := postgres.NewRegionRepository(pgPool.DB)
	cRepo := cache.NewRedisRegionCache(redisPool.Client)

	// 3. Eksekusi Impor
	ctx := context.Background()
	stats, err := RunImport(ctx, *filePath, *batchSize, *dryRun, repo, cRepo)
	if err != nil {
		log.Fatalf("[FATAL] Terjadi kesalahan saat import: %v", err)
	}

	stats.Duration = time.Since(start)

	// 4. Cetak Laporan
	printSummary(*filePath, *batchSize, *dryRun, stats)
}

// RunImport menjalankan parsing streaming CSV dan update batch
func RunImport(
	ctx context.Context,
	csvPath string,
	batchSize int,
	dryRun bool,
	repo domain.RegionRepository,
	cRepo domain.RegionCacheRepository,
) (*ImportStats, error) {
	cleanPath := filepath.Clean(csvPath)
	file, err := os.Open(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("gagal membuka berkas CSV: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(bufio.NewReader(file))
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1

	// Baca Header
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("gagal membaca header CSV: %w", err)
	}

	codeIdx := -1
	postalIdx := -1
	for idx, col := range header {
		cleanCol := strings.ToLower(strings.TrimSpace(col))
		if cleanCol == "code" {
			codeIdx = idx
		} else if cleanCol == "postal_code" || cleanCol == "postalcode" {
			postalIdx = idx
		}
	}

	if codeIdx == -1 || postalIdx == -1 {
		return nil, fmt.Errorf("format header CSV tidak valid: kolom 'code' dan 'postal_code' wajib ada (ditemukan: %v)", header)
	}

	stats := &ImportStats{}
	var batch []PostalItem
	var updatedCodes []string

	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			stats.InvalidFormat++
			continue
		}

		stats.TotalRead++
		if len(row) <= codeIdx || len(row) <= postalIdx {
			stats.InvalidFormat++
			continue
		}

		code := strings.TrimSpace(row[codeIdx])
		postalCode := strings.TrimSpace(row[postalIdx])

		if code == "" || !postalCodeRegex.MatchString(postalCode) {
			stats.InvalidFormat++
			continue
		}

		stats.ValidRows++
		batch = append(batch, PostalItem{
			Code:       code,
			PostalCode: postalCode,
		})

		if len(batch) >= batchSize {
			updated, err := flushBatch(ctx, batch, dryRun, repo)
			if err != nil {
				return nil, err
			}
			stats.UpdatedRows += updated
			if !dryRun {
				for _, it := range batch {
					updatedCodes = append(updatedCodes, it.Code)
				}
			}
			batch = batch[:0]
		}
	}

	// Flush sisa batch terakhir
	if len(batch) > 0 {
		updated, err := flushBatch(ctx, batch, dryRun, repo)
		if err != nil {
			return nil, err
		}
		stats.UpdatedRows += updated
		if !dryRun {
			for _, it := range batch {
				updatedCodes = append(updatedCodes, it.Code)
			}
		}
	}

	stats.SkippedRows = stats.ValidRows - int(stats.UpdatedRows)

	// Invalidate Cache Redis jika bukan dryRun dan ada data yang diperbarui
	if !dryRun && len(updatedCodes) > 0 && cRepo != nil {
		invalidateCache(ctx, updatedCodes, cRepo)
	}

	return stats, nil
}

func flushBatch(ctx context.Context, batch []PostalItem, dryRun bool, repo domain.RegionRepository) (int64, error) {
	if len(batch) == 0 {
		return 0, nil
	}

	codes := make([]string, len(batch))
	postals := make([]string, len(batch))
	for i, item := range batch {
		codes[i] = item.Code
		postals[i] = item.PostalCode
	}

	if dryRun {
		// Pada mode dry-run, kita hanya simulasikan hitung entitas yang cocok
		// atau cukup asumsikan baris valid
		return int64(len(batch)), nil
	}

	affected, err := repo.BatchUpdatePostalCodes(ctx, codes, postals)
	if err != nil {
		return 0, fmt.Errorf("gagal batch update: %w", err)
	}

	return affected, nil
}

func invalidateCache(ctx context.Context, codes []string, cRepo domain.RegionCacheRepository) {
	var keysToDelete []string
	for _, code := range codes {
		keysToDelete = append(keysToDelete,
			cRepo.KeyRegionDetail(code),
			fmt.Sprintf("geo:village:postal:%s", code),
		)
		// Hapus per 500 keys agar efisien
		if len(keysToDelete) >= 500 {
			_ = cRepo.Delete(ctx, keysToDelete...)
			keysToDelete = keysToDelete[:0]
		}
	}
	if len(keysToDelete) > 0 {
		_ = cRepo.Delete(ctx, keysToDelete...)
	}
}

func printSummary(filePath string, batchSize int, dryRun bool, stats *ImportStats) {
	modeStr := "COMMIT (Aktual)"
	if dryRun {
		modeStr = "SIMULASI (Dry-Run)"
	}

	rps := float64(0)
	if stats.Duration.Seconds() > 0 {
		rps = float64(stats.TotalRead) / stats.Duration.Seconds()
	}

	fmt.Println()
	fmt.Println("============================================================")
	fmt.Println("              LAPORAN IMPORT KODE POS CSV                   ")
	fmt.Println("============================================================")
	fmt.Printf("Berkas Sumber         : %s\n", filePath)
	fmt.Printf("Status Mode           : %s\n", modeStr)
	fmt.Printf("Ukuran Batch          : %d baris/chunk\n", batchSize)
	fmt.Println("------------------------------------------------------------")
	fmt.Printf("Total Baris Terbaca   : %d\n", stats.TotalRead)
	fmt.Printf("Format Valid          : %d\n", stats.ValidRows)
	fmt.Printf("Format Tidak Valid    : %d (Dilewati)\n", stats.InvalidFormat)
	fmt.Printf("Kelurahan Diperbarui  : %d\n", stats.UpdatedRows)
	fmt.Printf("Kode Tidak Ditemukan  : %d (Dilewati/Bukan Level 4)\n", stats.SkippedRows)
	fmt.Printf("Durasi Proses         : %v\n", stats.Duration)
	fmt.Printf("Kecepatan Rata-rata   : %.2f baris/detik\n", rps)
	fmt.Println("============================================================")
	if dryRun {
		fmt.Println("[INFO] Simulasi selesai. Gunakan tanpa flag -dry-run untuk commit perubahan.")
	} else {
		fmt.Println("[SUCCESS] Pembaruan massal kode pos berhasil diselesaikan!")
	}
	fmt.Println()
}
