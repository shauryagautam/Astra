package config

import (
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"time"
)

// AstraConfig is the root configuration struct for all Astra services.
type AstraConfig struct {
	App       AppConfig
	Database  DatabaseConfig
	Redis     RedisConfig
	Auth      AuthConfig
	OAuth2    OAuth2Config
	Storage   StorageConfig
	Mail      MailConfig
	Queue     QueueConfig
	Telemetry TelemetryConfig
	Assets    AssetConfig
	WS        WSConfig
}

// WSConfig holds WebSocket settings.
type WSConfig struct {
	AllowedOrigins []string `env:"WS_ALLOWED_ORIGINS"`
}

// AppConfig holds general application settings.
type AppConfig struct {
	Name            string        `env:"APP_NAME"`
	Environment     string        `env:"APP_ENV"`
	Host            string        `env:"HOST"`
	Port            int           `env:"PORT"`
	Debug           bool          `env:"APP_DEBUG"`
	Key             string        `env:"APP_KEY"`
	MaxBodySize     int64         `env:"APP_MAX_BODY_SIZE"`
	Version         string        `env:"APP_VERSION"`
	EncryptionKey   string        `env:"APP_ENCRYPTION_KEY"`
	AuditLogPath    string        `env:"AUDIT_LOG_PATH"`
	ShutdownTimeout time.Duration `env:"APP_SHUTDOWN_TIMEOUT"`
}

// DatabaseConfig holds Postgres connection settings, optimized for NeonDB.
type DatabaseConfig struct {
	URL             string        `env:"DATABASE_URL"`
	MaxConns        int32         `env:"DB_MAX_CONNS"`
	MinConns        int32         `env:"DB_MIN_CONNS"`
	MaxIdle         time.Duration `env:"DB_MAX_IDLE"`
	SSL             string        `env:"DB_SSL"`
	LogQueries      bool          `env:"DB_LOG_QUERIES"`
	SlowQueryThresh time.Duration `env:"DB_SLOW_QUERY_THRESH"`
}

// RedisConfig holds Redis connection settings.
type RedisConfig struct {
	URL            string   `env:"REDIS_URL"`
	Host           string   `env:"REDIS_HOST"`
	Port           int      `env:"REDIS_PORT"`
	Password       string   `env:"REDIS_PASSWORD"`
	DB             int      `env:"REDIS_DB"`
	MaxRetries     int      `env:"REDIS_MAX_RETRIES"`
	PoolSize       int      `env:"REDIS_POOL_SIZE"`
	UseSentinel    bool     `env:"REDIS_USE_SENTINEL"`
	SentinelMaster string   `env:"REDIS_SENTINEL_MASTER"`
	SentinelAddrs  []string `env:"REDIS_SENTINEL_ADDRS"`
	UseCluster     bool     `env:"REDIS_USE_CLUSTER"`
}

// AuthConfig holds authentication settings.
type AuthConfig struct {
	JWTSecret          string        `env:"JWT_SECRET"`
	JWTIssuer          string        `env:"JWT_ISSUER"`
	AccessTokenExpiry  time.Duration `env:"JWT_ACCESS_EXPIRY"`
	RefreshTokenExpiry time.Duration `env:"JWT_REFRESH_EXPIRY"`
}

// StorageConfig holds file storage settings.
type StorageConfig struct {
	Driver           string `env:"STORAGE_DRIVER"`
	LocalRoot        string `env:"STORAGE_LOCAL_ROOT"`
	S3Bucket         string `env:"S3_BUCKET"`
	S3Region         string `env:"S3_REGION"`
	S3Endpoint       string `env:"S3_ENDPOINT"`
	S3AccessKey      string `env:"S3_ACCESS_KEY"`
	S3SecretKey      string `env:"S3_SECRET_KEY"`
	S3ForcePathStyle bool   `env:"S3_FORCE_PATH_STYLE"`
}

// MailConfig holds mailer settings.
type MailConfig struct {
	Driver       string `env:"MAIL_DRIVER"`
	SMTPHost     string `env:"SMTP_HOST"`
	SMTPPort     int    `env:"SMTP_PORT"`
	SMTPUser     string `env:"SMTP_USER"`
	SMTPPassword string `env:"SMTP_PASSWORD"`
	SMTPFrom     string `env:"SMTP_FROM"`
	ResendAPIKey string `env:"RESEND_API_KEY"`
}

// QueueConfig holds background queue settings.
type QueueConfig struct {
	Driver      string   `env:"QUEUE_DRIVER"`
	Concurrency int      `env:"QUEUE_CONCURRENCY"`
	Prefix      string   `env:"QUEUE_PREFIX"`
	Queues      []string `env:"QUEUE_QUEUES"`
}

// TelemetryConfig holds OpenTelemetry settings.
type TelemetryConfig struct {
	Endpoint    string `env:"OTEL_EXPORTER_OTLP_ENDPOINT"`
	ServiceName string `env:"OTEL_SERVICE_NAME"`
}

// OAuth2Config holds OAuth2 provider configurations.
type OAuth2Config struct {
	Google  OAuth2ProviderEnvConfig
	GitHub  OAuth2ProviderEnvConfig
	Discord OAuth2ProviderEnvConfig
}

// OAuth2ProviderEnvConfig holds the env-loaded config for a single OAuth2 provider.
type OAuth2ProviderEnvConfig struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

// AssetConfig holds asset pipeline configuration.
type AssetConfig struct {
	Entrypoints []string // e.g. ["resources/js/app.js", "resources/css/app.css"]
	OutputDir   string   // e.g. "public/build"
	PublicPath  string   // e.g. "/build/"
	Minify      bool
	Sourcemap   bool
	Manifest    string // e.g. "public/build/manifest.json"
}

// Validate checks that all required AstraConfig fields are set.
// Call this at application startup to fail fast on misconfiguration.
func (c *AstraConfig) Validate() error {
	var errs []string

	// 1. App Security
	if c.App.Key == "" {
		errs = append(errs, "APP_KEY is required for security and sessions")
	} else if len(c.App.Key) < 32 {
		errs = append(errs, "APP_KEY must be at least 32 characters long")
	}

	// 2. Auth Security
	if c.Auth.JWTSecret == "" {
		if c.App.Key != "" {
			c.Auth.JWTSecret = c.App.Key
		} else {
			errs = append(errs, "JWT_SECRET is required (defaults to APP_KEY if set)")
		}
	} else if len(c.Auth.JWTSecret) < 32 {
		errs = append(errs, "JWT_SECRET must be at least 32 characters long")
	}

	// 3. Database
	if c.Database.URL == "" {
		errs = append(errs, "DATABASE_URL is required")
	}

	if len(errs) > 0 {
		return fmt.Errorf("astra config validation failed:\n  - %s",
			strings.Join(errs, "\n  - "))
	}

	if c.App.Environment == "production" {
		if err := c.ValidateProduction(); err != nil {
			return err
		}
	}

	return nil
}

// ValidateProduction performs stricter validation for production environments.
func (c *AstraConfig) ValidateProduction() error {
	var errs []string

	if c.App.Debug {
		errs = append(errs, "APP_DEBUG must be false in production")
	}

	if c.Database.SSL == "disable" {
		errs = append(errs, "DB_SSL cannot be 'disable' in production")
	}

	if len(errs) > 0 {
		return fmt.Errorf("production security check failed:\n  - %s",
			strings.Join(errs, "\n  - "))
	}
	return nil
}

// ValidateRequired returns an error if any of the named env vars are empty strings.
// Usage: cfg.ValidateRequired("STRIPE_KEY", "SENDGRID_KEY")
func (c *AstraConfig) ValidateRequired(keys ...string) error {
	var errs []string
	for _, k := range keys {
		key := strings.TrimSpace(k)
		if key == "" {
			errs = append(errs, "required config key cannot be empty")
			continue
		}
		if strings.TrimSpace(c.lookupValue(key)) == "" {
			errs = append(errs, fmt.Sprintf("required config key %q is empty", key))
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func (c *AstraConfig) lookupValue(key string) string {
	val := findEnvValue(reflect.ValueOf(c).Elem(), key)

	// Special fallbacks
	if key == "APP_ENCRYPTION_KEY" && val == "" {
		return c.App.Key
	}
	if key == "JWT_SECRET" && val == "" {
		if c.Auth.JWTSecret != "" {
			return c.Auth.JWTSecret
		}
		return c.App.Key
	}

	if val == "" {
		return os.Getenv(key)
	}
	return val
}

func findEnvValue(v reflect.Value, key string) string {
	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		structField := t.Field(i)
		tag := structField.Tag.Get("env")

		if tag == key {
			return fmt.Sprint(field.Interface())
		}

		if field.Kind() == reflect.Struct {
			if val := findEnvValue(field, key); val != "" {
				return val
			}
		}
	}
	return ""
}

// LoadFromEnv creates an AstraConfig populated from environment variables.
func LoadFromEnv(c *Config) *AstraConfig {
	return &AstraConfig{
		App: AppConfig{
			Name:            c.String("APP_NAME", "Astra App"),
			Environment:     c.String("APP_ENV", "development"),
			Host:            c.String("HOST", "0.0.0.0"),
			Port:            c.Int("PORT", 3333),
			Debug:           c.Bool("APP_DEBUG", true),
			Key:             c.String("APP_KEY", ""),
			MaxBodySize:     int64(c.Int("APP_MAX_BODY_SIZE", 10*1024*1024)),
			Version:         c.String("APP_VERSION", "1.0.0"),
			EncryptionKey:   c.String("APP_ENCRYPTION_KEY", c.String("APP_KEY", "")),
			AuditLogPath:    c.String("AUDIT_LOG_PATH", "storage/logs/audit.log"),
			ShutdownTimeout: c.Duration("APP_SHUTDOWN_TIMEOUT", 15*time.Second),
		},
		Database: DatabaseConfig{
			URL:             c.String("DATABASE_URL", ""),
			MaxConns:        c.Int32("DB_MAX_CONNS", 10),
			MinConns:        c.Int32("DB_MIN_CONNS", 2),
			MaxIdle:         c.Duration("DB_MAX_IDLE", 30*time.Second),
			SSL:             c.String("DB_SSL", "require"),
			LogQueries:      c.Bool("DB_LOG_QUERIES", false),
			SlowQueryThresh: c.Duration("DB_SLOW_QUERY_THRESH", 200*time.Millisecond),
		},
		Redis: RedisConfig{
			URL:        c.String("REDIS_URL", ""),
			Host:       c.String("REDIS_HOST", "127.0.0.1"),
			Port:       c.Int("REDIS_PORT", 6379),
			Password:   c.String("REDIS_PASSWORD", ""),
			DB:         c.Int("REDIS_DB", 0),
			MaxRetries: c.Int("REDIS_MAX_RETRIES", 3),
			PoolSize:   c.Int("REDIS_POOL_SIZE", 10),
			// Sentinel/Cluster
			UseSentinel:    c.Bool("REDIS_USE_SENTINEL", false),
			SentinelMaster: c.String("REDIS_SENTINEL_MASTER", "mymaster"),
			SentinelAddrs:  strings.Split(c.String("REDIS_SENTINEL_ADDRS", ""), ","),
			UseCluster:     c.Bool("REDIS_USE_CLUSTER", false),
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
			Queues:      strings.Split(c.String("QUEUE_QUEUES", "default"), ","),
		},
		Telemetry: TelemetryConfig{
			Endpoint:    c.String("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
			ServiceName: c.String("OTEL_SERVICE_NAME", c.String("APP_NAME", "Astra App")),
		},
		WS: WSConfig{
			AllowedOrigins: strings.Split(c.String("WS_ALLOWED_ORIGINS", ""), ","),
		},
	}
}
