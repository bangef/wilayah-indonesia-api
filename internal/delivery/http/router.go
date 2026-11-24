package http

import (
	"be_region_indonesia/config"
	_ "be_region_indonesia/docs/swagger"
	"be_region_indonesia/internal/delivery/http/handler"
	"be_region_indonesia/internal/delivery/http/middleware"
	"be_region_indonesia/pkg/validator"

	"github.com/labstack/echo/v4"
	echomw "github.com/labstack/echo/v4/middleware"
	"github.com/redis/go-redis/v9"
	echoSwagger "github.com/swaggo/echo-swagger"
)

type RouterConfig struct {
	App           *echo.Echo
	Cfg           *config.Config
	RedisClient   *redis.Client
	ReadHandler   *handler.RegionReadHandler
	WriteHandler  *handler.RegionWriteHandler
	HealthHandler *handler.HealthHandler
	AuthHandler   *handler.AuthHandler
}

func SetupRouter(rc RouterConfig) {
	e := rc.App

	// 1. Core Global Middleware
	e.Validator = validator.NewCustomValidator()
	e.Use(echomw.Recover())
	e.Use(echomw.RequestID())
	e.Use(echomw.Logger())

	// 2. Security Perimeter Middleware
	e.Use(middleware.SetupSecureHeaders())
	e.Use(middleware.SetupCORS(
		rc.Cfg.CORSAllowedOrigins,
		rc.Cfg.CORSAllowCreds,
		rc.Cfg.CORSMaxAgeSeconds,
	))

	// 3. Rate Limiters
	publicRateLimiter := middleware.RedisSlidingWindowRateLimiter(
		rc.RedisClient,
		rc.Cfg.RateLimitPublicRPM,
		"public",
	)
	writeRateLimiter := middleware.RedisSlidingWindowRateLimiter(
		rc.RedisClient,
		rc.Cfg.RateLimitAdminRPM,
		"admin_write",
	)
	authRateLimiter := middleware.RedisSlidingWindowRateLimiter(
		rc.RedisClient,
		rc.Cfg.RateLimitAuthRPM,
		"auth_login",
	)

	// Liveness & Readiness Probes (Root level, unthrottled for orchestrators/systemd)
	if rc.HealthHandler != nil {
		e.GET("/healthz", rc.HealthHandler.Healthz)
		e.GET("/readyz", rc.HealthHandler.Readyz)
	}

	// 4. Interactive Swagger / OpenAPI UI (Conditional via SWAGGER_ENABLED)
	if rc.Cfg != nil && rc.Cfg.SwaggerEnabled {
		e.GET("/swagger/*", echoSwagger.WrapHandler, publicRateLimiter)
	}

	// 5. API v1 Route Group
	v1 := e.Group("/api/v1")

	// ==========================================
	// AUTHENTICATION ROUTES (Login)
	// ==========================================
	if rc.AuthHandler != nil {
		auth := v1.Group("/auth", authRateLimiter)
		auth.POST("/login", rc.AuthHandler.Login)
	}

	// ==========================================
	// PUBLIC READ ROUTES (Cascading Queries)
	// ==========================================
	public := v1.Group("", publicRateLimiter)
	{
		// Level 1: Provinces
		public.GET("/provinces", rc.ReadHandler.GetProvinces)
		public.GET("/provinces/:code", rc.ReadHandler.GetProvinceByCode)

		// Level 2: Regencies by Province
		public.GET("/provinces/:code/regencies", rc.ReadHandler.GetRegenciesByProvince)
		public.GET("/regencies/:code", rc.ReadHandler.GetRegencyByCode)

		// Level 3: Districts by Regency
		public.GET("/regencies/:code/districts", rc.ReadHandler.GetDistrictsByRegency)
		public.GET("/districts/:code", rc.ReadHandler.GetDistrictByCode)

		// Level 4: Villages by District
		public.GET("/districts/:code/villages", rc.ReadHandler.GetVillagesByDistrict)
		public.GET("/villages/:code", rc.ReadHandler.GetVillageByCode)
		public.GET("/villages/:code/postal-code", rc.ReadHandler.GetPostalCodeByVillage)

		// Level 5: Postal Codes by Village
		public.GET("/villages/:code/postal-codes", rc.ReadHandler.GetPostalCodesByVillage)

		// Reverse Lookup: Get full hierarchy by 5-digit postal code
		public.GET("/postal-codes/:postal_code", rc.ReadHandler.GetPostalHierarchy)
	}

	// ==========================================
	// PROTECTED WRITE ROUTES (Admin CRUD Tier)
	// ==========================================
	adminAuth := middleware.JWTAuthMiddleware(rc.Cfg.JWTSecret, rc.RedisClient, "ADMIN", "SUPERADMIN")
	protected := v1.Group("", writeRateLimiter, adminAuth)
	{
		// Level 1: Province CRUD
		protected.POST("/provinces", rc.WriteHandler.CreateProvince)

		// Level 2: Regency based on Province
		protected.POST("/provinces/:code/regencies", rc.WriteHandler.CreateRegency)

		// Level 3: District based on Regency
		protected.POST("/regencies/:code/districts", rc.WriteHandler.CreateDistrict)

		// Level 4: Village based on District
		protected.POST("/districts/:code/villages", rc.WriteHandler.CreateVillage)

		// Level 5: Postal Code based on Village
		protected.POST("/villages/:code/postal-codes", rc.WriteHandler.CreatePostalCode)
		protected.PUT("/villages/:code/postal-code", rc.WriteHandler.UpdateVillagePostalCode)

		// General Update & Delete by Code
		protected.PUT("/regions/:code", rc.WriteHandler.UpdateRegion)
		protected.DELETE("/regions/:code", rc.WriteHandler.DeleteRegion)
	}
}
