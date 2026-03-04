package config

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// AstraConfig is the root configuration struct for all Astra services.
type AstraConfig struct {
	App       AppConfig
	Database  DatabaseConfig
	Redis     RedisConfig
	Auth      AuthConfig
	Storage   StorageConfig
	Mail      MailConfig
	Queue     QueueConfig
	Telemetry TelemetryConfig
}

// AppConfig holds general application settings.
type AppConfig struct {
	Name        string
	Environment string
	Host        string
	Port        int
	Debug       bool
	Key         string // Application secret key
}

// DatabaseConfig holds Postgres connection settings, optimized for NeonDB.
type DatabaseConfig struct {
	URL        string        // Full Postgres connection string (NeonDB compatible)
	MaxConns   int32         // default 10
	MinConns   int32         // default 2
	MaxIdle    time.Duration // default 30s (important for NeonDB serverless)
	SSL        string        // "require" for NeonDB, "disable" for local
	LogQueries bool          // dev only
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	URL        string
	Host       string
	Port       int
	Password   string
	DB         int
	MaxRetries int
	PoolSize   int
}

// AuthConfig holds authentication settings.
type AuthConfig struct {
	JWTSecret          string
	JWTIssuer          string
	AccessTokenExpiry  time.Duration
	RefreshTokenExpiry time.Duration
}

// StorageConfig holds file storage settings.
type StorageConfig struct {
	Driver           string // "local" or "s3"
	LocalRoot        string
	S3Bucket         string
	S3Region         string
	S3Endpoint       string // For Cloudflare R2 or MinIO
	S3AccessKey      string
	S3SecretKey      string
	S3ForcePathStyle bool
}

// MailConfig holds mailer settings.
type MailConfig struct {
	Driver       string // "smtp" or "resend"
	SMTPHost     string
	SMTPPort     int
	SMTPUser     string
	SMTPPassword string
	SMTPFrom     string
	ResendAPIKey string
}

// QueueConfig holds background queue settings.
type QueueConfig struct {
	Driver      string // "redis"
	Concurrency int
	Prefix      string
}

// TelemetryConfig holds OpenTelemetry settings.
type TelemetryConfig struct {
	Endpoint    string
	ServiceName string
}

// Validate checks that all required AstraConfig fields are set.
// Call this at application startup to fail fast on misconfiguration.
func (c *AstraConfig) Validate() error {
	var errs []string

	if c.Database.URL == "" {
		errs = append(errs, "DATABASE_URL is required")
	}
	if c.App.Key == "" {
		errs = append(errs, "APP_KEY is required — run: astra key:generate")
	}
	if c.Auth.JWTSecret == "" && c.App.Key != "" {
		// Fall back to APP_KEY if JWT_SECRET not explicitly set
		c.Auth.JWTSecret = c.App.Key
	}
	if c.Auth.JWTSecret == "" {
		errs = append(errs, "JWT_SECRET is required (or set APP_KEY as fallback)")
	}

	if len(errs) > 0 {
		return fmt.Errorf("astra config validation failed:\n  - %s",
			strings.Join(errs, "\n  - "))
	}
	return nil
}

// ValidateRequired returns an error if any of the named env vars are empty strings.
// Usage: cfg.ValidateRequired("STRIPE_KEY", "SENDGRID_KEY")
func (c *AstraConfig) ValidateRequired(keys ...string) error {
	var errs []string
	for _, k := range keys {
		if k == "" {
			errs = append(errs, fmt.Sprintf("required config key %q is empty", k))
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

// LoadFromEnv creates an AstraConfig populated from environment variables.
func LoadFromEnv(c *Config) *AstraConfig {
	return &AstraConfig{
		App: AppConfig{
			Name:        c.String("APP_NAME", "Astra App"),
			Environment: c.String("APP_ENV", "development"),
			Host:        c.String("HOST", "0.0.0.0"),
			Port:        c.Int("PORT", 3333),
			Debug:       c.Bool("APP_DEBUG", true),
			Key:         c.String("APP_KEY", ""),
		},
		Database: DatabaseConfig{
			URL:        c.String("DATABASE_URL", ""),
			MaxConns:   c.Int32("DB_MAX_CONNS", 10),
			MinConns:   c.Int32("DB_MIN_CONNS", 2),
			MaxIdle:    c.Duration("DB_MAX_IDLE", 30*time.Second),
			SSL:        c.String("DB_SSL", "require"),
			LogQueries: c.Bool("DB_LOG_QUERIES", false),
		},
		Redis: RedisConfig{
			URL:        c.String("REDIS_URL", ""),
			Host:       c.String("REDIS_HOST", "127.0.0.1"),
			Port:       c.Int("REDIS_PORT", 6379),
			Password:   c.String("REDIS_PASSWORD", ""),
			DB:         c.Int("REDIS_DB", 0),
			MaxRetries: c.Int("REDIS_MAX_RETRIES", 3),
			PoolSize:   c.Int("REDIS_POOL_SIZE", 10),
		},
		Auth: AuthConfig{
			JWTSecret:          c.String("JWT_SECRET", ""),
			JWTIssuer:          c.String("JWT_ISSUER", "astra"),
			AccessTokenExpiry:  c.Duration("JWT_ACCESS_EXPIRY", 15*time.Minute),
			RefreshTokenExpiry: c.Duration("JWT_REFRESH_EXPIRY", 7*24*time.Hour),
		},
		Storage: StorageConfig{
			Driver:           c.String("STORAGE_DRIVER", "local"),
			LocalRoot:        c.String("STORAGE_LOCAL_ROOT", "./storage"),
			S3Bucket:         c.String("S3_BUCKET", ""),
			S3Region:         c.String("S3_REGION", "us-east-1"),
			S3Endpoint:       c.String("S3_ENDPOINT", ""),
			S3AccessKey:      c.String("S3_ACCESS_KEY", ""),
			S3SecretKey:      c.String("S3_SECRET_KEY", ""),
			S3ForcePathStyle: c.Bool("S3_FORCE_PATH_STYLE", false),
		},
		Mail: MailConfig{
			Driver:       c.String("MAIL_DRIVER", "smtp"),
			SMTPHost:     c.String("SMTP_HOST", "localhost"),
			SMTPPort:     c.Int("SMTP_PORT", 587),
			SMTPUser:     c.String("SMTP_USER", ""),
			SMTPPassword: c.String("SMTP_PASSWORD", ""),
			SMTPFrom:     c.String("SMTP_FROM", "noreply@example.com"),
			ResendAPIKey: c.String("RESEND_API_KEY", ""),
		},
		Queue: QueueConfig{
			Driver:      c.String("QUEUE_DRIVER", "redis"),
			Concurrency: c.Int("QUEUE_CONCURRENCY", 5),
			Prefix:      c.String("QUEUE_PREFIX", "astra:queue:"),
		},
		Telemetry: TelemetryConfig{
			Endpoint:    c.String("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
			ServiceName: c.String("OTEL_SERVICE_NAME", c.String("APP_NAME", "Astra App")),
		},
	}
}
