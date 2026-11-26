// @title           API Wilayah Administrasi Indonesia
// @version         1.0.0
// @description     Layanan backend berkinerja tinggi untuk data wilayah administrasi Republik Indonesia (Provinsi, Kabupaten/Kota, Kecamatan, Kelurahan/Desa, dan Kode Pos) dengan proteksi anti-stampede Singleflight dan Redis Caching.
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
// @description Masukkan token JWT dengan format: Bearer <token_admin>

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

	// 5. Inisialisasi Usecase Layer
	regionUsecase := usecase.NewRegionUsecase(regionRepo, regionCache, cfg.RedisDefaultTTL)
	authUsecase := usecase.NewAuthUsecase(cfg)

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
		ReadHandler:   readHandler,
		WriteHandler:  writeHandler,
		HealthHandler: healthHandler,
		AuthHandler:   authHandler,
	})

	serverAddr := fmt.Sprintf(":%d", cfg.AppPort)

	// 8. Menjalankan Server di Goroutine terpisah
	go func() {
		log.Printf("[READY] Server siap melayani request di http://127.0.0.1%s/api/v1\n", serverAddr)
		log.Printf("[READY] Health probes aktif di http://127.0.0.1%s/healthz dan /readyz\n", serverAddr)
		if err := e.Start(serverAddr); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("[FATAL] Server HTTP crash: %v", err)
		}
	}()

	// 9. Graceful Shutdown: Menangkap sinyal interupsi OS (SIGINT, SIGTERM)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	sig := <-quit
	log.Printf("[SHUTDOWN] Menerima sinyal OS (%v). Menutup server secara graceful...\n", sig)

	// Berikan batas waktu 15 detik untuk menyelesaikan request in-flight
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := e.Shutdown(ctx); err != nil {
		log.Printf("[SHUTDOWN_ERROR] Gagal mematikan HTTP server secara mulus: %v\n", err)
	} else {
		log.Println("[SHUTDOWN] HTTP listener dan in-flight request telah selesai dibersihkan.")
	}

	log.Println("[SHUTDOWN] Server exiting successfully.")
}
