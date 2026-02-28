package config

// CorsConfig holds CORS configuration.
// Mirrors AdonisJS's config/cors.ts.
type CorsConfig struct {
	// Enabled toggles CORS handling.
	Enabled bool

	// Origin specifies allowed origins ("*" for all).
	Origin []string

	// Methods specifies allowed HTTP methods.
	Methods []string

	// Headers specifies allowed request headers.
	Headers []string

	// ExposeHeaders specifies headers exposed to the client.
	ExposeHeaders []string

	// Credentials allows cookies/auth in CORS.
	Credentials bool

	// MaxAge is the preflight cache duration in seconds.
	MaxAge int
}

// DefaultCorsConfig returns sensible defaults.
func DefaultCorsConfig() CorsConfig {
	return CorsConfig{
		Enabled:     true,
		Origin:      []string{"*"},
		Methods:     []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE"},
		Headers:     []string{"Content-Type", "Accept", "Authorization", "X-Requested-With"},
		Credentials: true,
		MaxAge:      86400,
	}
}

// HashConfig holds hashing configuration.
// Mirrors AdonisJS's config/hash.ts.
type HashConfig struct {
	// Default driver: "argon2" or "bcrypt".
	Driver string

	// Argon2 settings
	Argon2 Argon2Config

	// Bcrypt settings
	Bcrypt BcryptConfig
}

// Argon2Config holds Argon2id settings.
type Argon2Config struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

// BcryptConfig holds Bcrypt settings.
type BcryptConfig struct {
	Rounds int
}

// DefaultHashConfig returns sensible defaults.
func DefaultHashConfig() HashConfig {
	return HashConfig{
		Driver: "argon2",
		Argon2: Argon2Config{
			Memory:      65536,
			Iterations:  3,
			Parallelism: 2,
			SaltLength:  16,
			KeyLength:   32,
		},
		Bcrypt: BcryptConfig{
			Rounds: 10,
		},
	}
}

// AuthConfig holds authentication configuration.
// Mirrors AdonisJS's config/auth.ts.
type AuthConfig struct {
	// Guard is the default authentication guard.
	Guard string

	// Guards holds all configured guards.
	Guards map[string]GuardConfig
}

// GuardConfig configures a single auth guard.
type GuardConfig struct {
	// Driver: "jwt", "oat" (opaque access token), "session".
	Driver string

	// Provider is the user provider name.
	Provider string

	// JWT-specific settings
	JWT JWTConfig
}

// JWTConfig holds JWT guard settings.
type JWTConfig struct {
	// Secret is the JWT signing key.
	Secret string

	// Expiry is the token lifetime in seconds.
	Expiry int

	// Issuer is the JWT issuer claim.
	Issuer string
}

// DefaultAuthConfig returns sensible defaults.
func DefaultAuthConfig() AuthConfig {
	return AuthConfig{
		Guard: "api",
		Guards: map[string]GuardConfig{
			"api": {
				Driver:   "jwt",
				Provider: "user",
				JWT: JWTConfig{
					Secret: "change-me-in-production",
					Expiry: 86400, // 24 hours
					Issuer: "adonis",
				},
			},
		},
	}
}

// RedisConfig holds Redis configuration.
// Mirrors AdonisJS's config/redis.ts.
type RedisConfig struct {
	// Connection is the default Redis connection.
	Connection string

	// Connections holds all Redis connections.
	Connections map[string]RedisConnectionConfig
}

// RedisConnectionConfig holds a single Redis connection's settings.
type RedisConnectionConfig struct {
	Host     string
	Port     int
	Password string
	DB       int
}

// DefaultRedisConfig returns sensible defaults.
func DefaultRedisConfig() RedisConfig {
	return RedisConfig{
		Connection: "local",
		Connections: map[string]RedisConnectionConfig{
			"local": {
				Host:     "127.0.0.1",
				Port:     6379,
				Password: "",
				DB:       0,
			},
		},
	}
}
