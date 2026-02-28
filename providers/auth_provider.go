package providers

import (
	"time"

	auth "github.com/shaurya/adonis/app/Auth"
	hash "github.com/shaurya/adonis/app/Hash"
	"github.com/shaurya/adonis/contracts"
)

// AuthProvider registers hash and auth services into the container.
// Mirrors AdonisJS's AuthProvider.
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
	p.App.Singleton("Adonis/Core/Hash", func(c contracts.ContainerContract) (any, error) {
		env := c.Use("Env").(*EnvManager)
		driver := env.Get("HASH_DRIVER", "argon2")
		return hash.NewManager(driver, hash.DefaultArgon2Config(), hash.DefaultBcryptConfig()), nil
	})
	p.App.Alias("Hash", "Adonis/Core/Hash")

	// Register the Auth manager
	p.App.Singleton("Adonis/Core/Auth", func(c contracts.ContainerContract) (any, error) {
		env := c.Use("Env").(*EnvManager)
		defaultGuard := env.Get("AUTH_GUARD", "api")
		return auth.NewAuthManager(defaultGuard), nil
	})
	p.App.Alias("Auth", "Adonis/Core/Auth")

	// Register the in-memory token store (swap for Redis in production)
	p.App.Singleton("Adonis/Core/TokenStore", func(c contracts.ContainerContract) (any, error) {
		return auth.NewMemoryTokenStore(), nil
	})
	p.App.Alias("TokenStore", "Adonis/Core/TokenStore")

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
	jwtGuard := auth.NewJWTGuard(auth.JWTConfig{
		Secret: jwtSecret,
		Expiry: jwtExpiry,
		Issuer: "adonis",
	}, nil) // User provides their own UserProvider

	oatGuard := auth.NewOATGuard(jwtExpiry, tokenStore, nil)

	authMgr.RegisterGuard("jwt", jwtGuard)
	authMgr.RegisterGuard("api", oatGuard)

	return nil
}
