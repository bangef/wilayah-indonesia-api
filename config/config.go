package config

import (
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

type Config struct {
	// 1. Application Settings
	AppName            string `mapstructure:"APP_NAME"`
	AppEnv             string `mapstructure:"APP_ENV"`
	AppPort            int    `mapstructure:"APP_PORT"`
	AppTimeoutSeconds  int    `mapstructure:"APP_TIMEOUT_SECONDS"`
	AppDebug           bool   `mapstructure:"APP_DEBUG"`
	LogLevel           string `mapstructure:"LOG_LEVEL"`

	// 2. Database Settings (PostgreSQL)
	DBHost             string `mapstructure:"DB_HOST"`
	DBPort             int    `mapstructure:"DB_PORT"`
	DBUser             string `mapstructure:"DB_USER"`
	DBPassword         string `mapstructure:"DB_PASSWORD"`
	DBName             string `mapstructure:"DB_NAME"`
	DBSSLMode          string `mapstructure:"DB_SSLMODE"`
	DBMaxOpenConns     int    `mapstructure:"DB_MAX_OPEN_CONNS"`
	DBMaxIdleConns     int    `mapstructure:"DB_MAX_IDLE_CONNS"`
	DBConnMaxLifetimeM int    `mapstructure:"DB_CONN_MAX_LIFETIME_MINUTES"`
	DBConnMaxIdleTimeM int    `mapstructure:"DB_CONN_MAX_IDLE_TIME_MINUTES"`

	// 3. Cache Settings (Redis)
	RedisHost          string `mapstructure:"REDIS_HOST"`
	RedisPort          int    `mapstructure:"REDIS_PORT"`
	RedisPassword      string `mapstructure:"REDIS_PASSWORD"`
	RedisDB            int    `mapstructure:"REDIS_DB"`
	RedisPoolSize      int    `mapstructure:"REDIS_POOL_SIZE"`
	RedisDefaultTTL    int    `mapstructure:"REDIS_DEFAULT_TTL_HOURS"`

	// 4. Security & JWT
	JWTSecret            string `mapstructure:"JWT_SECRET"`
	JWTExpirationM       int    `mapstructure:"JWT_EXPIRATION_MINUTES"`
	JWTIssuer            string `mapstructure:"JWT_ISSUER"`
	JWTAudience          string `mapstructure:"JWT_AUDIENCE"`
	AdminDefaultUsername string `mapstructure:"ADMIN_DEFAULT_USERNAME"`
	AdminDefaultPassword string `mapstructure:"ADMIN_DEFAULT_PASSWORD"`

	// 5. Rate Limiting
	RateLimitEnabled     bool   `mapstructure:"RATE_LIMIT_ENABLED"`
	RateLimitPublicRPM   int    `mapstructure:"RATE_LIMIT_PUBLIC_RPM"`
	RateLimitSearchRPM   int    `mapstructure:"RATE_LIMIT_SEARCH_RPM"`
	RateLimitAdminRPM    int    `mapstructure:"RATE_LIMIT_ADMIN_RPM"`
	RateLimitAuthRPM     int    `mapstructure:"RATE_LIMIT_AUTH_RPM"`

	// 6. CORS
	CORSAllowedOrigins string `mapstructure:"CORS_ALLOWED_ORIGINS"`
	CORSAllowCreds     bool   `mapstructure:"CORS_ALLOW_CREDENTIALS"`
	CORSMaxAgeSeconds  int    `mapstructure:"CORS_MAX_AGE_SECONDS"`

	// 7. Swagger / OpenAPI Documentation
	SwaggerEnabled     bool   `mapstructure:"SWAGGER_ENABLED"`
}

// LoadConfig memuat konfigurasi dari path file .env dan environment variables
func LoadConfig(envPath string) (*Config, error) {
	v := viper.New()

	// Default fallback values
	setDefaults(v)

	// Bind environment keys
	bindEnvs(v)

	// Jika ada file env spesifik, muat dan set ke viper
	if envPath != "" {
		envMap, err := godotenv.Read(envPath)
		if err == nil {
			for key, val := range envMap {
				v.Set(key, val)
			}
		}
	} else {
		_ = godotenv.Load()
		v.AutomaticEnv()
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal configuration: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return &cfg, nil
}

// MustLoadConfig memuat konfigurasi dan memicu fatal error jika terjadi kegagalan (Fail-Fast)
func MustLoadConfig(envPath string) *Config {
	cfg, err := LoadConfig(envPath)
	if err != nil {
		log.Fatalf("[FATAL] Configuration initialization failed: %v", err)
	}
	return cfg
}

func (c *Config) Validate() error {
	var missing []string

	if strings.TrimSpace(c.AppName) == "" {
		missing = append(missing, "APP_NAME")
	}
	if c.AppPort <= 0 || c.AppPort > 65535 {
		missing = append(missing, "APP_PORT (invalid port number)")
	}
	if strings.TrimSpace(c.DBHost) == "" {
		missing = append(missing, "DB_HOST")
	}
	if c.DBPort <= 0 {
		missing = append(missing, "DB_PORT")
	}
	if strings.TrimSpace(c.DBUser) == "" {
		missing = append(missing, "DB_USER")
	}
	if strings.TrimSpace(c.DBName) == "" {
		missing = append(missing, "DB_NAME")
	}
	if strings.TrimSpace(c.RedisHost) == "" {
		missing = append(missing, "REDIS_HOST")
	}
	if c.RedisPort <= 0 {
		missing = append(missing, "REDIS_PORT")
	}

	// In production, require strict credentials
	if c.AppEnv == "production" {
		if strings.TrimSpace(c.DBPassword) == "" {
			missing = append(missing, "DB_PASSWORD (required in production)")
		}
		if len(c.JWTSecret) < 32 {
			missing = append(missing, "JWT_SECRET (must be >= 32 characters in production)")
		}
	}

	if len(missing) > 0 {
		return errors.New("missing or invalid required environment variables: " + strings.Join(missing, ", "))
	}

	return nil
}

func setDefaults(v *viper.Viper) {
	v.SetDefault("APP_NAME", "be_region_indonesia")
	v.SetDefault("APP_ENV", "development")
	v.SetDefault("APP_PORT", 8080)
	v.SetDefault("APP_TIMEOUT_SECONDS", 30)
	v.SetDefault("APP_DEBUG", true)
	v.SetDefault("LOG_LEVEL", "debug")

	v.SetDefault("DB_HOST", "127.0.0.1")
	v.SetDefault("DB_PORT", 5432)
	v.SetDefault("DB_USER", "postgres")
	v.SetDefault("DB_NAME", "region_db")
	v.SetDefault("DB_SSLMODE", "disable")
	v.SetDefault("DB_MAX_OPEN_CONNS", 50)
	v.SetDefault("DB_MAX_IDLE_CONNS", 10)
	v.SetDefault("DB_CONN_MAX_LIFETIME_MINUTES", 60)
	v.SetDefault("DB_CONN_MAX_IDLE_TIME_MINUTES", 15)

	v.SetDefault("REDIS_HOST", "127.0.0.1")
	v.SetDefault("REDIS_PORT", 6379)
	v.SetDefault("REDIS_DB", 0)
	v.SetDefault("REDIS_POOL_SIZE", 100)
	v.SetDefault("REDIS_DEFAULT_TTL_HOURS", 168)

	v.SetDefault("JWT_EXPIRATION_MINUTES", 15)
	v.SetDefault("JWT_ISSUER", "be-region-auth-service")
	v.SetDefault("JWT_AUDIENCE", "be-region-api")
	v.SetDefault("ADMIN_DEFAULT_USERNAME", "admin")
	v.SetDefault("ADMIN_DEFAULT_PASSWORD", "AdminRegional2026!")

	v.SetDefault("RATE_LIMIT_ENABLED", true)
	v.SetDefault("RATE_LIMIT_PUBLIC_RPM", 120)
	v.SetDefault("RATE_LIMIT_SEARCH_RPM", 60)
	v.SetDefault("RATE_LIMIT_ADMIN_RPM", 30)
	v.SetDefault("RATE_LIMIT_AUTH_RPM", 10)

	v.SetDefault("CORS_ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:5173")
	v.SetDefault("CORS_ALLOW_CREDENTIALS", true)
	v.SetDefault("CORS_MAX_AGE_SECONDS", 86400)

	v.SetDefault("SWAGGER_ENABLED", true)
}

func bindEnvs(v *viper.Viper) {
	keys := []string{
		"APP_NAME", "APP_ENV", "APP_PORT", "APP_TIMEOUT_SECONDS", "APP_DEBUG", "LOG_LEVEL",
		"DB_HOST", "DB_PORT", "DB_USER", "DB_PASSWORD", "DB_NAME", "DB_SSLMODE",
		"DB_MAX_OPEN_CONNS", "DB_MAX_IDLE_CONNS", "DB_CONN_MAX_LIFETIME_MINUTES", "DB_CONN_MAX_IDLE_TIME_MINUTES",
		"REDIS_HOST", "REDIS_PORT", "REDIS_PASSWORD", "REDIS_DB", "REDIS_POOL_SIZE", "REDIS_DEFAULT_TTL_HOURS",
		"JWT_SECRET", "JWT_EXPIRATION_MINUTES", "JWT_ISSUER", "JWT_AUDIENCE",
		"ADMIN_DEFAULT_USERNAME", "ADMIN_DEFAULT_PASSWORD",
		"RATE_LIMIT_ENABLED", "RATE_LIMIT_PUBLIC_RPM", "RATE_LIMIT_SEARCH_RPM", "RATE_LIMIT_ADMIN_RPM", "RATE_LIMIT_AUTH_RPM",
		"CORS_ALLOWED_ORIGINS", "CORS_ALLOW_CREDENTIALS", "CORS_MAX_AGE_SECONDS",
		"SWAGGER_ENABLED",
	}

	for _, key := range keys {
		_ = v.BindEnv(key)
	}
}
