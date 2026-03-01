package providers

import (
	"time"

	auth "github.com/shaurya/astra/app/Auth"
	hash "github.com/shaurya/astra/app/Hash"
	"github.com/shaurya/astra/contracts"
	"gorm.io/gorm"
)

// AuthProvider registers hash and auth services into the container.
// Mirrors Astra's AuthProvider.
type AuthProvider struct {
	BaseProvider
}

// NewAuthProvider creates a new AuthProvider.
func NewAuthProvider(app contracts.ApplicationContract) *AuthProvider {
	return &AuthProvider{
		BaseProvider: NewBaseProvider(app),
	}
}

// Register binds Hash and Auth into the container.
func (p *AuthProvider) Register() error {
	// Register the Hash manager
	p.App.Singleton("Astra/Core/Hash", func(c contracts.ContainerContract) (any, error) {
		env := c.Use("Env").(*EnvManager)
		driver := env.Get("HASH_DRIVER", "argon2")
		return hash.NewManager(driver, hash.DefaultArgon2Config(), hash.DefaultBcryptConfig()), nil
	})
	p.App.Alias("Hash", "Astra/Core/Hash")

	// Register the Auth manager
	p.App.Singleton("Astra/Core/Auth", func(c contracts.ContainerContract) (any, error) {
		env := c.Use("Env").(*EnvManager)
		defaultGuard := env.Get("AUTH_GUARD", "api")
		return auth.NewAuthManager(defaultGuard), nil
	})
	p.App.Alias("Auth", "Astra/Core/Auth")

	// Register the token store (Database-backed for production, Memory for development)
	p.App.Singleton("Astra/Core/TokenStore", func(c contracts.ContainerContract) (any, error) {
		if c.HasBinding("Astra/Lucid/Database") {
			db := c.Use("Astra/Lucid/Database").(*gorm.DB)
			return auth.NewDatabaseTokenStore(db), nil
		}
		return auth.NewMemoryTokenStore(), nil
	})
	p.App.Alias("TokenStore", "Astra/Core/TokenStore")

	// Register the Blacklist service
	p.App.Singleton("Astra/Core/Blacklist", func(c contracts.ContainerContract) (any, error) {
		redis := c.Use("Redis").(contracts.RedisContract).Connection("local")
		return auth.NewRedisBlacklist(redis), nil
	})
	p.App.Alias("Blacklist", "Astra/Core/Blacklist")

	return nil
}

// Boot wires up the JWT guard with the auth manager.
func (p *AuthProvider) Boot() error {
	authMgr := p.App.Use("Auth").(*auth.AuthManager)
	tokenStore := p.App.Use("TokenStore").(auth.TokenStore)

	env := p.App.Use("Env").(*EnvManager)
	jwtSecret := env.Get("APP_KEY", "change-me-in-production")
	jwtExpiry := 24 * time.Hour

	// Create JWT guard (requires a UserProvider to be registered by the app)
	// For now, register a placeholder that the user's app will replace
	blacklist := p.App.Use("Blacklist").(contracts.BlacklistContract)
	jwtGuard := auth.NewJWTGuard(auth.JWTConfig{
		Secret: jwtSecret,
		Expiry: jwtExpiry,
		Issuer: "astra",
	}, nil, blacklist) // User provides their own UserProvider

	oatGuard := auth.NewOATGuard(jwtExpiry, tokenStore, nil)

	authMgr.RegisterGuard("jwt", jwtGuard)
	authMgr.RegisterGuard("api", oatGuard)

	return nil
}
