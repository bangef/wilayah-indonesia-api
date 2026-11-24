package handler

import (
	"context"
	"database/sql"
	"net/http"
	"time"

	"be_region_indonesia/pkg/response"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
)

type HealthHandler struct {
	db  *sql.DB
	rdb *redis.Client
}

func NewHealthHandler(db *sql.DB, rdb *redis.Client) *HealthHandler {
	return &HealthHandler{
		db:  db,
		rdb: rdb,
	}
}

// Healthz adalah Liveness Probe: Memastikan server HTTP Echo aktif dan merespon 200 OK.
// @Summary      Liveness Health Probe
// @Description  Memeriksa ketersediaan proses HTTP server.
// @Tags         System Health
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Router       /healthz [get]
func (h *HealthHandler) Healthz(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]interface{}{
		"status":    "AKTIF",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

// Readyz adalah Readiness Probe: Memeriksa konektivitas langsung ke PostgreSQL lokal dan Redis lokal.
// Jika salah satu gagal, kembalikan 503 Service Unavailable.
// @Summary      Readiness Health Probe
// @Description  Memeriksa kesiapan dependensi eksternal (PostgreSQL & Redis).
// @Tags         System Health
// @Produce      json
// @Success      200  {object}  map[string]interface{}
// @Failure      503  {object}  map[string]interface{}
// @Router       /readyz [get]
func (h *HealthHandler) Readyz(c echo.Context) error {
	ctx, cancel := context.WithTimeout(c.Request().Context(), 2*time.Second)
	defer cancel()

	checks := make(map[string]string)
	isHealthy := true

	// Check PostgreSQL
	if h.db != nil {
		if err := h.db.PingContext(ctx); err != nil {
			checks["postgres"] = "TERPUTUS: " + err.Error()
			isHealthy = false
		} else {
			checks["postgres"] = "TERHUBUNG"
		}
	} else {
		checks["postgres"] = "BELUM_DIKONFIGURASI"
		isHealthy = false
	}

	// Check Redis
	if h.rdb != nil {
		if err := h.rdb.Ping(ctx).Err(); err != nil {
			checks["redis"] = "TERPUTUS: " + err.Error()
			isHealthy = false
		} else {
			checks["redis"] = "TERHUBUNG"
		}
	} else {
		checks["redis"] = "BELUM_DIKONFIGURASI"
		isHealthy = false
	}

	if !isHealthy {
		return c.JSON(http.StatusServiceUnavailable, map[string]interface{}{
			"status":    "BELUM_SIAP",
			"timestamp": time.Now().UTC().Format(time.RFC3339),
			"checks":    checks,
		})
	}

	return response.Success(c, http.StatusOK, "Layanan backend siap melayani permintaan.", map[string]interface{}{
		"status":    "SIAP",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"checks":    checks,
	}, nil)
}
