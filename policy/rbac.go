package policy

import (
	"strings"
)

// Role represents a user role with fixed permissions.
type Role struct {
	Name        string
	Permissions []string
}

// HasPermission checks if the role has the given permission.
func (r *Role) HasPermission(permission string) bool {
	for _, p := range r.Permissions {
		if p == "*" || p == permission {
			return true
		}
		// Support wildcard suffix e.g. "users.*"
		if strings.HasSuffix(p, ".*") {
			prefix := strings.TrimSuffix(p, ".*")
			if strings.HasPrefix(permission, prefix+".") {
				return true
			}
		}
	}
	return false
}

// RBAC holds the roles and their definitions.
type RBAC struct {
	roles map[string]*Role
}

// NewRBAC creates a new RBAC manager.
func NewRBAC() *RBAC {
	return &RBAC{
		roles: make(map[string]*Role),
	}
}

// DefineRole adds a role definition.
func (r *RBAC) DefineRole(name string, permissions []string) {
	r.roles[name] = &Role{
		Name:        name,
		Permissions: permissions,
	}
}

// Can checks if any of the given roles have the required permission.
func (r *RBAC) Can(userRoles []string, permission string) bool {
	for _, roleName := range userRoles {
		if role, ok := r.roles[roleName]; ok {
			if role.HasPermission(permission) {
				return true
			}
		}
	}
	return false
}
