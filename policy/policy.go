// Package policy provides a resource-level authorization gate.
// It is separate from authentication: Auth answers "who are you?",
// Policy answers "are you allowed to do this?".
package policy

import (
	"fmt"
	"reflect"
)

// PolicyFunc is a function that evaluates whether a user can perform an
// action on a given subject. Return true to allow, false to deny.
type PolicyFunc func(user any, subject any) bool

// Gate holds all registered policy functions.
type Gate struct {
	policies map[string]PolicyFunc
}

// New creates a new Gate.
func New() *Gate {
	return &Gate{
		policies: make(map[string]PolicyFunc),
	}
}

// DefaultGate is the package-level default gate.
var DefaultGate = New()

// Register associates a PolicyFunc with an action and a subject type.
//
// The key is "<action>:<TypeName>", e.g. "update:Post" or "delete:Comment".
// Pass nil as subject to create a global action (e.g. "create:Post" without a specific instance).
//
// Example:
//
//	gate.Register("update", (*Post)(nil), func(user, subject any) bool {
//	    u := user.(*User)
//	    p := subject.(*Post)
//	    return p.UserID == u.ID || u.IsAdmin
//	})
func (g *Gate) Register(action string, subjectType any, fn PolicyFunc) {
	key := policyKey(action, subjectType)
	g.policies[key] = fn
}

// Allows returns true if the user is permitted to perform action on subject.
func (g *Gate) Allows(user any, action string, subject any) bool {
	key := policyKey(action, subject)
	fn, ok := g.policies[key]
	if !ok {
		// Deny by default if no policy is registered
		return false
	}
	return fn(user, subject)
}

// Denies returns true if the user is NOT permitted to perform action on subject.
func (g *Gate) Denies(user any, action string, subject any) bool {
	return !g.Allows(user, action, subject)
}

// Authorize returns an error if the user is not permitted to perform action on subject.
// The error is a PolicyDeniedError suitable for returning from an HTTP handler.
func (g *Gate) Authorize(user any, action string, subject any) error {
	if g.Allows(user, action, subject) {
		return nil
	}
	return &PolicyDeniedError{
		Action:  action,
		Subject: fmt.Sprintf("%T", subject),
	}
}

// PolicyDeniedError is returned by Gate.Authorize when a policy denies access.
type PolicyDeniedError struct {
	Action  string
	Subject string
	Message string
}

func (e *PolicyDeniedError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("not authorized to %s %s", e.Action, e.Subject)
}

// HTTPStatus implements the http.HTTPError interface so it maps to 403 Forbidden.
func (e *PolicyDeniedError) HTTPStatus() int { return 403 }

// policyKey builds the map key from an action and a subject instance/type.
func policyKey(action string, subject any) string {
	if subject == nil {
		return action + ":any"
	}
	t := reflect.TypeOf(subject)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return action + ":" + t.Name()
}

// ─── Package-level helpers using DefaultGate ─────────────────────────────────

// Register registers a policy on the DefaultGate.
func Register(action string, subjectType any, fn PolicyFunc) {
	DefaultGate.Register(action, subjectType, fn)
}

// Allows checks a policy on the DefaultGate.
func Allows(user any, action string, subject any) bool {
	return DefaultGate.Allows(user, action, subject)
}

// Authorize checks a policy on the DefaultGate and returns an error on denial.
func Authorize(user any, action string, subject any) error {
	return DefaultGate.Authorize(user, action, subject)
}
