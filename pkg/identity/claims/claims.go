package claims

// AuthClaims holds the authenticated user's information extracted from authentication guards.
type AuthClaims struct {
	UserID string
	Email  string
	Claims map[string]any
}
