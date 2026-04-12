package auth

import (
	identityclaims "github.com/shauryagautam/Astra/pkg/identity/claims"
)

// Gate defines the authorization interface.
type Gate interface {
	Allows(user *identityclaims.AuthClaims, action string, subject any) bool
}
