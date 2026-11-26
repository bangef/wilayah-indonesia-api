// @title           API Wilayah Administrasi Indonesia
// @version         1.0.0
// @description     Layanan backend berkinerja tinggi untuk data wilayah administrasi Republik Indonesia (Provinsi, Kabupaten/Kota, Kecamatan, Kelurahan/Desa, dan Kode Pos) dengan otentikasi dual-token dan otorisasi dinamis PBAC / Granular RBAC.
// @termsOfService  https://github.com/bangef/be_region_indonesia

// @contact.name    Tim Engineering Backend
// @contact.url     https://github.com/bangef/be_region_indonesia/issues
// @contact.email   engineer@example.com

// @license.name    MIT
// @license.url     https://opensource.org/licenses/MIT

// @host            localhost:8080
// @BasePath        /api/v1
// @schemes         http https

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Masukkan token JWT dengan format: Bearer <access_token>

package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"be_region_indonesia/config"
	delivery "be_region_indonesia/internal/delivery/http"
	"be_region_indonesia/internal/delivery/http/handler"
	"be_region_indonesia/internal/repository/cache"
	"be_region_indonesia/internal/repository/postgres"
	"be_region_indonesia/internal/usecase"
	"be_region_indonesia/pkg/database"
	"be_region_indonesia/pkg/logger"

	"github.com/labstack/echo/v4"
)

func main() {
	// 1. Inisialisasi Konfigurasi (.env)
	cfg := config.MustLoadConfig(".env")

	// Inisialisasi Structured Logger (log/slog) ke os.Stdout
	logger.InitLogger(cfg.AppName, cfg.AppEnv)
	log.Printf("[BOOT] Memulai %s (%s mode) pada port %d...\n", cfg.AppName, cfg.AppEnv, cfg.AppPort)

	// 2. Inisialisasi Database Connection Pool (PostgreSQL)
	pgPool, err := database.NewPostgresConnection(cfg)
	if err != nil {
		log.Fatalf("[FATAL] Gagal menghubungkan ke PostgreSQL: %v", err)
	}
	defer pgPool.Close()
	log.Println("[BOOT] PostgreSQL connection pool terinisialisasi.")

	// 3. Inisialisasi Cache Client Pool (Redis)
	redisPool, err := database.NewRedisConnection(cfg)
	if err != nil {
		log.Fatalf("[FATAL] Gagal menghubungkan ke Redis: %v", err)
	}
	defer redisPool.Close()
	log.Println("[BOOT] Redis connection pool terinisialisasi.")

	// 4. Inisialisasi Repositories
	regionRepo := postgres.NewRegionRepository(pgPool.DB)
	regionCache := cache.NewRedisRegionCache(redisPool.Client)
	authRepo := postgres.NewAuthRepository(pgPool.DB)
	authCache := cache.NewRedisAuthCache(redisPool.Client)

	// 5. Inisialisasi Usecase Layer
	regionUsecase := usecase.NewRegionUsecase(regionRepo, regionCache, cfg.RedisDefaultTTL)
	authUsecase := usecase.NewAuthUsecase(cfg, authRepo, authCache)

	// 6. Inisialisasi HTTP Handlers
	readHandler := handler.NewRegionReadHandler(regionUsecase)
	writeHandler := handler.NewRegionWriteHandler(regionUsecase)
	healthHandler := handler.NewHealthHandler(pgPool.DB, redisPool.Client)
	authHandler := handler.NewAuthHandler(authUsecase)

	// 7. Inisialisasi Echo Server & Router
	e := echo.New()
	e.HideBanner = true

	delivery.SetupRouter(delivery.RouterConfig{
		App:           e,
		Cfg:           cfg,
		RedisClient:   redisPool.Client,
		AuthRepo:      authRepo,
		AuthCache:     authCache,
		ReadHandler:   readHandler,
		WriteHandler:  writeHandler,
		HealthHandler: healthHandler,
		AuthHandler:   authHandler,
	})

	serverAddr := fmt.Sprintf(":%d", cfg.AppPort)

	// 8. Menjalankan Server secara Non-Blocking (Goroutine)
	go func() {
		if err := e.Start(serverAddr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[FATAL] Server HTTP gagal berjalan: %v", err)
		}
	}()
	log.Printf("[READY] Server aktif mendengarkan pada %s\n", serverAddr)

	// 9. Graceful Shutdown Listener
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Println("[SHUTDOWN] Sinyal penghentian diterima, memulai graceful shutdown...")

	// Memberikan batas waktu toleransi maksimum 15 detik bagi request aktif
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := e.Shutdown(ctx); err != nil {
		log.Printf("[ERROR] Gagal melakukan graceful shutdown server HTTP: %v\n", err)
	} else {
		log.Println("[SHUTDOWN] Seluruh request aktif telah selesai diproses.")
	}

	log.Println("[EXIT] Aplikasi be_region_indonesia berhasil dimatikan secara bersih.")
}
