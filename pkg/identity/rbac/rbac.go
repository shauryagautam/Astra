package rbac

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"
	identityclaims "github.com/shauryagautam/Astra/pkg/identity/claims"
)


// Permission represents a specific permission
type Permission struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Resource    string            `json:"resource"`
	Action      string            `json:"action"`
	Attributes  map[string]string `json:"attributes,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// Role represents a role with permissions
type Role struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Permissions []string          `json:"permissions"` // Permission IDs
	Attributes  map[string]string `json:"attributes,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// UserRole represents the assignment of a role to a user
type UserRole struct {
	UserID    string    `json:"user_id"`
	RoleID    string    `json:"role_id"`
	Context   string    `json:"context,omitempty"` // Optional context for multi-tenancy
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Policy represents an access control policy
type Policy struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Effect      string                 `json:"effect"` // "allow" or "deny"
	Actions     []string               `json:"actions"`
	Resources   []string               `json:"resources"`
	Conditions  map[string]interface{} `json:"conditions,omitempty"`
	Priority    int                    `json:"priority"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// AccessRequest represents an access control request
type AccessRequest struct {
	UserID     string                 `json:"user_id"`
	Resource   string                 `json:"resource"`
	Action     string                 `json:"action"`
	Context    map[string]interface{} `json:"context,omitempty"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

// AccessResult represents the result of an access decision
type AccessResult struct {
	Allowed   bool                   `json:"allowed"`
	Reason    string                 `json:"reason"`
	Policies  []string               `json:"policies"` // Applied policy IDs
	Roles     []string               `json:"roles"`    // User's roles that were evaluated
	Duration  time.Duration          `json:"duration"` // Time taken to evaluate
	Timestamp time.Time              `json:"timestamp"`
}

// Storage interface for RBAC data persistence
type Storage interface {
	// Permissions
	GetPermission(ctx context.Context, id string) (*Permission, error)
	GetPermissionByName(ctx context.Context, name string) (*Permission, error)
	ListPermissions(ctx context.Context) ([]*Permission, error)
	CreatePermission(ctx context.Context, perm *Permission) error
	UpdatePermission(ctx context.Context, perm *Permission) error
	DeletePermission(ctx context.Context, id string) error

	// Roles
	GetRole(ctx context.Context, id string) (*Role, error)
	GetRoleByName(ctx context.Context, name string) (*Role, error)
	ListRoles(ctx context.Context) ([]*Role, error)
	CreateRole(ctx context.Context, role *Role) error
	UpdateRole(ctx context.Context, role *Role) error
	DeleteRole(ctx context.Context, id string) error

	// User Roles
	AssignRole(ctx context.Context, userRole *UserRole) error
	RemoveRole(ctx context.Context, userID, roleID string) error
	GetUserRoles(ctx context.Context, userID string) ([]*UserRole, error)
	GetUsersInRole(ctx context.Context, roleID string) ([]*UserRole, error)

	// Policies
	GetPolicy(ctx context.Context, id string) (*Policy, error)
	ListPolicies(ctx context.Context) ([]*Policy, error)
	CreatePolicy(ctx context.Context, policy *Policy) error
	UpdatePolicy(ctx context.Context, policy *Policy) error
	DeletePolicy(ctx context.Context, id string) error
}

// RBAC is the main Role-Based Access Control system
type RBAC struct {
	storage    Storage
	cache      Cache
	logger     *slog.Logger
	mu         sync.RWMutex
	policies   map[string]*Policy
	roles      map[string]*Role
	permissions map[string]*Permission
}


// Cache interface for caching RBAC data
type Cache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{}, ttl time.Duration)
	Delete(key string)
	Clear()
}

// Logger interface for logging RBAC events
type Logger interface {
	Info(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Debug(msg string, fields ...interface{})
}

// NewRBAC creates a new RBAC system
func NewRBAC(storage Storage, cache Cache, logger *slog.Logger) *RBAC {
	return &RBAC{
		storage:     storage,
		cache:       cache,
		logger:      logger,
		policies:    make(map[string]*Policy),
		roles:       make(map[string]*Role),
		permissions: make(map[string]*Permission),
	}
}


// Allows implements the Gate interface.
func (rbac *RBAC) Allows(user *identityclaims.AuthClaims, action string, subject any) bool {
	if user == nil {
		return false
	}

	resource := "*"
	switch s := subject.(type) {
	case string:
		resource = s
	case interface{ ResourceName() string }:
		resource = s.ResourceName()
	case fmt.Stringer:
		resource = s.String()
	}

	req := &AccessRequest{
		UserID:   user.UserID,
		Resource: resource,
		Action:   action,
	}

	// CheckAccess is thread-safe and has internal caching
	result, err := rbac.CheckAccess(context.Background(), req)
	if err != nil {
		return false
	}

	return result.Allowed
}

// CheckAccess checks if a user has access to a resource/action
func (rbac *RBAC) CheckAccess(ctx context.Context, req *AccessRequest) (*AccessResult, error) {
	start := time.Now()
	result := &AccessResult{
		Timestamp: start,
	}

	// Get user roles
	userRoles, err := rbac.getUserRoles(ctx, req.UserID)
	if err != nil {
		result.Reason = fmt.Sprintf("Failed to get user roles: %v", err)
		return result, err
	}

	// Extract role IDs
	roleIDs := make([]string, len(userRoles))
	for i, ur := range userRoles {
		roleIDs[i] = ur.RoleID
		result.Roles = append(result.Roles, ur.RoleID)
	}

	// Check policies first (higher priority)
	allowed, policies, reason := rbac.evaluatePolicies(ctx, req, roleIDs)
	result.Allowed = allowed
	result.Policies = policies
	result.Reason = reason

	// If no policy explicitly denies access, check role-based permissions
	if result.Allowed || result.Reason == "" {
		roleAllowed, roleReason := rbac.evaluateRoles(ctx, req, roleIDs)
		if !result.Allowed && roleAllowed {
			result.Allowed = true
			result.Reason = roleReason
		} else if !roleAllowed && result.Reason == "" {
			result.Reason = roleReason
		}
	}

	result.Duration = time.Since(start)

	rbac.logger.Info("Access decision",
		"user_id", req.UserID,
		"resource", req.Resource,
		"action", req.Action,
		"allowed", result.Allowed,
		"reason", result.Reason,
		"duration", result.Duration,
	)

	return result, nil
}

// evaluatePolicies evaluates access control policies
func (rbac *RBAC) evaluatePolicies(ctx context.Context, req *AccessRequest, roleIDs []string) (bool, []string, string) {
	policies, err := rbac.getAllPolicies(ctx)
	if err != nil {
		return false, nil, fmt.Sprintf("Failed to get policies: %v", err)
	}

	var appliedPolicies []string
	var lastDenyReason string

	// Sort policies by priority (higher first)
	sortedPolicies := make([]*Policy, len(policies))
	copy(sortedPolicies, policies)
	// Simple sort by priority (in real implementation, use proper sorting)
	for i := 0; i < len(sortedPolicies)-1; i++ {
		for j := i + 1; j < len(sortedPolicies); j++ {
			if sortedPolicies[i].Priority < sortedPolicies[j].Priority {
				sortedPolicies[i], sortedPolicies[j] = sortedPolicies[j], sortedPolicies[i]
			}
		}
	}

	for _, policy := range sortedPolicies {
		if rbac.policyMatches(ctx, policy, req, roleIDs) {
			appliedPolicies = append(appliedPolicies, policy.ID)
			
			if policy.Effect == "deny" {
				lastDenyReason = fmt.Sprintf("Denied by policy: %s", policy.Name)
				// Deny policies take precedence
				return false, appliedPolicies, lastDenyReason
			} else if policy.Effect == "allow" {
				return true, appliedPolicies, fmt.Sprintf("Allowed by policy: %s", policy.Name)
			}
		}
	}

	return false, appliedPolicies, lastDenyReason
}

// policyMatches checks if a policy matches the access request
func (rbac *RBAC) policyMatches(ctx context.Context, policy *Policy, req *AccessRequest, roleIDs []string) bool {
	// Check action match
	if !rbac.matchesPattern(policy.Actions, req.Action) {
		return false
	}

	// Check resource match
	if !rbac.matchesPattern(policy.Resources, req.Resource) {
		return false
	}

	// Check conditions
	for key, condition := range policy.Conditions {
		if !rbac.evaluateCondition(key, condition, req, roleIDs) {
			return false
		}
	}

	return true
}

// matchesPattern checks if a value matches any pattern in the list
func (rbac *RBAC) matchesPattern(patterns []string, value string) bool {
	for _, pattern := range patterns {
		if rbac.patternMatches(pattern, value) {
			return true
		}
	}
	return false
}

// patternMatches checks if a pattern matches a value (supports wildcards)
func (rbac *RBAC) patternMatches(pattern, value string) bool {
	if pattern == "*" {
		return true
	}
	
	if strings.Contains(pattern, "*") {
		// Simple wildcard matching
		parts := strings.Split(pattern, "*")
		if len(parts) == 2 {
			prefix := parts[0]
			suffix := parts[1]
			return strings.HasPrefix(value, prefix) && strings.HasSuffix(value, suffix)
		}
	}
	
	return pattern == value
}

// evaluateCondition evaluates a policy condition
func (rbac *RBAC) evaluateCondition(key string, condition interface{}, req *AccessRequest, roleIDs []string) bool {
	switch key {
	case "time":
		return rbac.evaluateTimeCondition(condition)
	case "ip":
		return rbac.evaluateIPCondition(condition, req)
	case "role":
		return rbac.evaluateRoleCondition(condition, roleIDs)
	case "attribute":
		return rbac.evaluateAttributeCondition(condition, req)
	default:
		return true // Unknown conditions pass by default
	}
}

// evaluateTimeCondition evaluates time-based conditions
func (rbac *RBAC) evaluateTimeCondition(condition interface{}) bool {
	// Simple implementation - check if current time is within allowed hours
	now := time.Now()
	hour := now.Hour()
	
	if hours, ok := condition.([]interface{}); ok {
		for _, h := range hours {
			if hInt, ok := h.(float64); ok {
				if int(hInt) == hour {
					return true
				}
			}
		}
	}
	
	return false
}

// evaluateIPCondition evaluates IP-based conditions
func (rbac *RBAC) evaluateIPCondition(condition interface{}, req *AccessRequest) bool {
	// Simple implementation - check if IP is in allowed list
	if req.Context == nil {
		return false
	}
	
	ip, ok := req.Context["ip"].(string)
	if !ok {
		return false
	}
	
	if allowedIPs, ok := condition.([]interface{}); ok {
		for _, allowedIP := range allowedIPs {
			if allowedIPStr, ok := allowedIP.(string); ok {
				if allowedIPStr == ip || allowedIPStr == "*" {
					return true
				}
			}
		}
	}
	
	return false
}

// evaluateRoleCondition evaluates role-based conditions
func (rbac *RBAC) evaluateRoleCondition(condition interface{}, roleIDs []string) bool {
	if requiredRoles, ok := condition.([]interface{}); ok {
		for _, requiredRole := range requiredRoles {
			if requiredRoleStr, ok := requiredRole.(string); ok {
				for _, userRoleID := range roleIDs {
					if userRoleID == requiredRoleStr {
						return true
					}
				}
			}
		}
	}
	return false
}

// evaluateAttributeCondition evaluates attribute-based conditions
func (rbac *RBAC) evaluateAttributeCondition(condition interface{}, req *AccessRequest) bool {
	if req.Attributes == nil {
		return false
	}
	
	if conditions, ok := condition.(map[string]interface{}); ok {
		for key, expectedValue := range conditions {
			if actualValue, exists := req.Attributes[key]; !exists || actualValue != expectedValue {
				return false
			}
		}
	}
	
	return true
}

// evaluateRoles evaluates role-based permissions
func (rbac *RBAC) evaluateRoles(ctx context.Context, req *AccessRequest, roleIDs []string) (bool, string) {
	for _, roleID := range roleIDs {
		role, err := rbac.getRole(ctx, roleID)
		if err != nil {
			continue
		}

		// Check if role has the required permission
		for _, permID := range role.Permissions {
			perm, err := rbac.getPermission(ctx, permID)
			if err != nil {
				continue
			}

			if rbac.permissionMatches(perm, req) {
				return true, fmt.Sprintf("Allowed by role '%s' with permission '%s'", role.Name, perm.Name)
			}
		}
	}

	return false, "Access denied: insufficient permissions"
}

// permissionMatches checks if a permission matches the access request
func (rbac *RBAC) permissionMatches(perm *Permission, req *AccessRequest) bool {
	// Check resource match
	if perm.Resource != "*" && perm.Resource != req.Resource {
		if !rbac.patternMatches(perm.Resource, req.Resource) {
			return false
		}
	}

	// Check action match
	if perm.Action != "*" && perm.Action != req.Action {
		if !rbac.patternMatches(perm.Action, req.Action) {
			return false
		}
	}

	return true
}

// Helper methods with caching
func (rbac *RBAC) getUserRoles(ctx context.Context, userID string) ([]*UserRole, error) {
	cacheKey := fmt.Sprintf("user_roles:%s", userID)
	if cached, found := rbac.cache.Get(cacheKey); found {
		if roles, ok := cached.([]*UserRole); ok {
			return roles, nil
		}
	}

	roles, err := rbac.storage.GetUserRoles(ctx, userID)
	if err != nil {
		return nil, err
	}

	rbac.cache.Set(cacheKey, roles, 5*time.Minute)
	return roles, nil
}

func (rbac *RBAC) getAllPolicies(ctx context.Context) ([]*Policy, error) {
	cacheKey := "all_policies"
	if cached, found := rbac.cache.Get(cacheKey); found {
		if policies, ok := cached.([]*Policy); ok {
			return policies, nil
		}
	}

	policies, err := rbac.storage.ListPolicies(ctx)
	if err != nil {
		return nil, err
	}

	rbac.cache.Set(cacheKey, policies, 10*time.Minute)
	return policies, nil
}

func (rbac *RBAC) getRole(ctx context.Context, roleID string) (*Role, error) {
	cacheKey := fmt.Sprintf("role:%s", roleID)
	if cached, found := rbac.cache.Get(cacheKey); found {
		if role, ok := cached.(*Role); ok {
			return role, nil
		}
	}

	role, err := rbac.storage.GetRole(ctx, roleID)
	if err != nil {
		return nil, err
	}

	rbac.cache.Set(cacheKey, role, 10*time.Minute)
	return role, nil
}

func (rbac *RBAC) getPermission(ctx context.Context, permID string) (*Permission, error) {
	cacheKey := fmt.Sprintf("permission:%s", permID)
	if cached, found := rbac.cache.Get(cacheKey); found {
		if perm, ok := cached.(*Permission); ok {
			return perm, nil
		}
	}

	perm, err := rbac.storage.GetPermission(ctx, permID)
	if err != nil {
		return nil, err
	}

	rbac.cache.Set(cacheKey, perm, 10*time.Minute)
	return perm, nil
}

// Permission management methods
func (rbac *RBAC) CreatePermission(ctx context.Context, perm *Permission) error {
	if err := rbac.storage.CreatePermission(ctx, perm); err != nil {
		return err
	}
	
	rbac.cache.Clear() // Clear cache
	rbac.logger.Info("Permission created", "permission_id", perm.ID, "name", perm.Name)
	return nil
}

func (rbac *RBAC) UpdatePermission(ctx context.Context, perm *Permission) error {
	if err := rbac.storage.UpdatePermission(ctx, perm); err != nil {
		return err
	}
	
	rbac.cache.Clear() // Clear cache
	rbac.logger.Info("Permission updated", "permission_id", perm.ID, "name", perm.Name)
	return nil
}

func (rbac *RBAC) DeletePermission(ctx context.Context, id string) error {
	if err := rbac.storage.DeletePermission(ctx, id); err != nil {
		return err
	}
	
	rbac.cache.Clear() // Clear cache
	rbac.logger.Info("Permission deleted", "permission_id", id)
	return nil
}

// Role management methods
func (rbac *RBAC) CreateRole(ctx context.Context, role *Role) error {
	if err := rbac.storage.CreateRole(ctx, role); err != nil {
		return err
	}
	
	rbac.cache.Clear() // Clear cache
	rbac.logger.Info("Role created", "role_id", role.ID, "name", role.Name)
	return nil
}

func (rbac *RBAC) UpdateRole(ctx context.Context, role *Role) error {
	if err := rbac.storage.UpdateRole(ctx, role); err != nil {
		return err
	}
	
	rbac.cache.Clear() // Clear cache
	rbac.logger.Info("Role updated", "role_id", role.ID, "name", role.Name)
	return nil
}

func (rbac *RBAC) DeleteRole(ctx context.Context, id string) error {
	if err := rbac.storage.DeleteRole(ctx, id); err != nil {
		return err
	}
	
	rbac.cache.Clear() // Clear cache
	rbac.logger.Info("Role deleted", "role_id", id)
	return nil
}

func (rbac *RBAC) AssignRole(ctx context.Context, userRole *UserRole) error {
	if err := rbac.storage.AssignRole(ctx, userRole); err != nil {
		return err
	}
	
	// Clear user role cache
	cacheKey := fmt.Sprintf("user_roles:%s", userRole.UserID)
	rbac.cache.Delete(cacheKey)
	
	rbac.logger.Info("Role assigned", "user_id", userRole.UserID, "role_id", userRole.RoleID)
	return nil
}

func (rbac *RBAC) RemoveRole(ctx context.Context, userID, roleID string) error {
	if err := rbac.storage.RemoveRole(ctx, userID, roleID); err != nil {
		return err
	}
	
	// Clear user role cache
	cacheKey := fmt.Sprintf("user_roles:%s", userID)
	rbac.cache.Delete(cacheKey)
	
	rbac.logger.Info("Role removed", "user_id", userID, "role_id", roleID)
	return nil
}

// Policy management methods
func (rbac *RBAC) CreatePolicy(ctx context.Context, policy *Policy) error {
	if err := rbac.storage.CreatePolicy(ctx, policy); err != nil {
		return err
	}
	
	rbac.cache.Clear() // Clear cache
	rbac.logger.Info("Policy created", "policy_id", policy.ID, "name", policy.Name)
	return nil
}

func (rbac *RBAC) UpdatePolicy(ctx context.Context, policy *Policy) error {
	if err := rbac.storage.UpdatePolicy(ctx, policy); err != nil {
		return err
	}
	
	rbac.cache.Clear() // Clear cache
	rbac.logger.Info("Policy updated", "policy_id", policy.ID, "name", policy.Name)
	return nil
}

func (rbac *RBAC) DeletePolicy(ctx context.Context, id string) error {
	if err := rbac.storage.DeletePolicy(ctx, id); err != nil {
		return err
	}
	
	rbac.cache.Clear() // Clear cache
	rbac.logger.Info("Policy deleted", "policy_id", id)
	return nil
}
