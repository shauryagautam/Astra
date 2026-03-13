package auth

// AuthClaims holds the authenticated user's information extracted from JWT.
type AuthClaims struct {
	UserID string
	Email  string
	Claims map[string]any
}
