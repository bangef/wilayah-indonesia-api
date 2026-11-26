package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"text/tabwriter"
	"time"

	"be_region_indonesia/internal/pkg/migrator"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	action := flag.String("action", "up", "Aksi migrasi: up, rollback, refresh, reset, version, status")
	steps := flag.Int("steps", 1, "Jumlah step/batch untuk di-rollback (digunakan pada action rollback)")
	targetVersion := flag.String("target", "", "Target versi migrasi (digunakan pada action version, e.g. 000002)")
	seed := flag.Bool("seed", false, "Jalankan seeder wilayah setelah operasi refresh sukses")
	force := flag.Bool("force", false, "Paksa eksekusi tindakan berbahaya (seperti reset/refresh) di environment production")
	migrationsDir := flag.String("dir", "migrations", "Direktori tempat berkas SQL migrasi berada")
	flag.Parse()

	_ = godotenv.Load()

	appEnv := getEnv("APP_ENV", "development")
	if (*action == "refresh" || *action == "reset") && appEnv == "production" && !*force {
		log.Fatalf("[FATAL] Aksi '%s' ditolak pada APP_ENV=production tanpa flag -force=true!", *action)
	}

	dbHost := getEnv("DB_HOST", "127.0.0.1")
	dbPort := getEnv("DB_PORT", "5432")
	dbUser := getEnv("DB_USER", "postgres")
	dbPass := getEnv("DB_PASSWORD", "postgres")
	dbName := getEnv("DB_NAME", "region_db")
	dbSSL := getEnv("DB_SSLMODE", "disable")

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		dbHost, dbPort, dbUser, dbPass, dbName, dbSSL)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("[FATAL] Gagal membuka koneksi database: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("[FATAL] Gagal menghubungkan ke PostgreSQL: %v", err)
	}

	engine := migrator.NewMigrator(db, *migrationsDir)

	switch *action {
	case "up":
		count, err := engine.Up(ctx)
		if err != nil {
			log.Fatalf("[ERROR] Eksekusi migrasi up gagal: %v", err)
		}
		fmt.Printf("[SUCCESS] Berhasil mengaplikasikan %d berkas migrasi.\n", count)

	case "rollback":
		count, err := engine.Rollback(ctx, *steps)
		if err != nil {
			log.Fatalf("[ERROR] Eksekusi rollback gagal: %v", err)
		}
		fmt.Printf("[SUCCESS] Berhasil me-rollback %d berkas migrasi (%d step/batch).\n", count, *steps)

	case "refresh":
		count, err := engine.Refresh(ctx)
		if err != nil {
			log.Fatalf("[ERROR] Eksekusi refresh gagal: %v", err)
		}
		fmt.Printf("[SUCCESS] Skema database berhasil di-refresh (%d migrasi diaplikasikan).\n", count)

		if *seed {
			fmt.Println("[SEEDER] Menjalankan seeder data wilayah BPS...")
			cmd := exec.Command("go", "run", "cmd/seeder/main.go")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err != nil {
				log.Fatalf("[ERROR] Eksekusi seeder gagal setelah refresh: %v", err)
			}
			fmt.Println("[SUCCESS] Seeder wilayah berhasil diselesaikan.")
		}

	case "reset":
		count, err := engine.Reset(ctx)
		if err != nil {
			log.Fatalf("[ERROR] Eksekusi reset gagal: %v", err)
		}
		fmt.Printf("[SUCCESS] Seluruh migrasi database berhasil di-reset (%d berkas di-rollback).\n", count)

	case "version":
		if *targetVersion == "" {
			log.Fatalf("[FATAL] Flag -target wajib disertakan untuk action version (contoh: -target=000002)")
		}
		if err := engine.ToVersion(ctx, *targetVersion); err != nil {
			log.Fatalf("[ERROR] Migrasi ke target versi %s gagal: %v", *targetVersion, err)
		}

	case "status":
		statuses, err := engine.Status(ctx)
		if err != nil {
			log.Fatalf("[ERROR] Gagal mengambil status migrasi: %v", err)
		}
		printStatusTable(statuses)

	default:
		log.Fatalf("[FATAL] Aksi '%s' tidak dikenali. Pilih: up, rollback, refresh, reset, version, status", *action)
	}
}

func printStatusTable(statuses []migrator.MigrationStatus) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "STATUS\tVERSION\tMIGRATION NAME\tBATCH\tAPPLIED AT")
	fmt.Fprintln(w, "------\t-------\t--------------\t-----\t----------")
	for _, s := range statuses {
		batchStr := "-"
		if s.Batch != nil {
			batchStr = fmt.Sprintf("%d", *s.Batch)
		}
		appliedAtStr := "-"
		if s.AppliedAt != nil {
			appliedAtStr = *s.AppliedAt
		}
		statusFormatted := s.Status
		if s.Status == "APPLIED" {
			statusFormatted = "\033[32m[APPLIED]\033[0m"
		} else {
			statusFormatted = "\033[33m[PENDING]\033[0m"
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", statusFormatted, s.Version, s.Name, batchStr, appliedAtStr)
	}
	w.Flush()
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
