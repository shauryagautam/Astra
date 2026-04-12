package multitenancy

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Tenant represents a tenant in the system
type Tenant struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Slug        string                 `json:"slug"`
	Domain      string                 `json:"domain"`
	Status      TenantStatus           `json:"status"`
	Plan        string                 `json:"plan"`
	Settings    map[string]interface{} `json:"settings"`
	Metadata    map[string]interface{} `json:"metadata"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	ExpiresAt   *time.Time             `json:"expires_at"`
	Limits      *TenantLimits          `json:"limits"`
}

// TenantStatus represents tenant status
type TenantStatus string

const (
	TenantStatusActive    TenantStatus = "active"
	TenantStatusInactive  TenantStatus = "inactive"
	TenantStatusSuspended TenantStatus = "suspended"
	TenantStatusTrial     TenantStatus = "trial"
)

// TenantLimits represents resource limits for a tenant
type TenantLimits struct {
	MaxUsers       int `json:"max_users"`
	MaxStorage     int `json:"max_storage"`     // in MB
	MaxAPIRequests int `json:"max_api_requests"` // per day
	MaxConnections int `json:"max_connections"`
}

// TenantContext contains tenant information for the current request
type TenantContext struct {
	Tenant   *Tenant
	UserID   string
	IsAdmin  bool
	IsOwner  bool
	Roles    []string
	Metadata map[string]interface{}
}

// Storage interface for tenant persistence
type Storage interface {
	// Tenants
	CreateTenant(ctx context.Context, tenant *Tenant) error
	GetTenant(ctx context.Context, id string) (*Tenant, error)
	GetTenantByDomain(ctx context.Context, domain string) (*Tenant, error)
	GetTenantBySlug(ctx context.Context, slug string) (*Tenant, error)
	ListTenants(ctx context.Context) ([]*Tenant, error)
	UpdateTenant(ctx context.Context, tenant *Tenant) error
	DeleteTenant(ctx context.Context, id string) error

	// Tenant users
	AddTenantUser(ctx context.Context, tenantID, userID string, roles []string) error
	RemoveTenantUser(ctx context.Context, tenantID, userID string) error
	GetTenantUsers(ctx context.Context, tenantID string) ([]*TenantUser, error)
	GetUserTenants(ctx context.Context, userID string) ([]*Tenant, error)

	// Tenant usage
	GetTenantUsage(ctx context.Context, tenantID string) (*TenantUsage, error)
	UpdateTenantUsage(ctx context.Context, tenantID string, usage *TenantUsage) error
}

// TenantUser represents a user in a tenant
type TenantUser struct {
	TenantID string    `json:"tenant_id"`
	UserID   string    `json:"user_id"`
	Roles    []string  `json:"roles"`
	JoinedAt time.Time `json:"joined_at"`
}

// TenantUsage represents resource usage for a tenant
type TenantUsage struct {
	TenantID       string    `json:"tenant_id"`
	Users          int       `json:"users"`
	StorageUsed    int       `json:"storage_used"`    // in MB
	APIRequests    int       `json:"api_requests"`    // today
	ActiveSessions int       `json:"active_sessions"`
	LastReset      time.Time `json:"last_reset"`
}

// Manager manages multi-tenancy
type Manager struct {
	storage    Storage
	cache      Cache
	logger     Logger
	config     Config
	tenants    map[string]*Tenant
	mu         sync.RWMutex
}

// Cache interface for tenant caching
type Cache interface {
	Get(key string) (interface{}, bool)
	Set(key string, value interface{}, ttl time.Duration)
	Delete(key string)
	Clear()
}

// Logger interface for logging
type Logger interface {
	Info(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Debug(msg string, fields ...interface{})
}

// Config represents multi-tenancy configuration
type Config struct {
	DefaultPlan      string        `json:"default_plan"`
	TrialDuration    time.Duration `json:"trial_duration"`
	CacheTTL         time.Duration `json:"cache_ttl"`
	EnableSubdomains bool          `json:"enable_subdomains"`
	DefaultDomain    string        `json:"default_domain"`
}

// NewManager creates a new multi-tenancy manager
func NewManager(storage Storage, cache Cache, logger Logger, config Config) *Manager {
	return &Manager{
		storage: storage,
		cache:   cache,
		logger:  logger,
		config:  config,
		tenants: make(map[string]*Tenant),
	}
}

// CreateTenant creates a new tenant
func (m *Manager) CreateTenant(ctx context.Context, name, domain string, ownerID string) (*Tenant, error) {
	// Generate tenant slug
	slug := m.generateSlug(name)

	// Check if slug is available
	if existing, err := m.storage.GetTenantBySlug(ctx, slug); err == nil && existing != nil {
		return nil, fmt.Errorf("tenant slug '%s' is already taken", slug)
	}

	// Check if domain is available
	if domain != "" {
		if existing, err := m.storage.GetTenantByDomain(ctx, domain); err == nil && existing != nil {
			return nil, fmt.Errorf("domain '%s' is already taken", domain)
		}
	}

	// Create tenant
	tenant := &Tenant{
		ID:       m.generateID(),
		Name:     name,
		Slug:     slug,
		Domain:   domain,
		Status:   TenantStatusTrial,
		Plan:     m.config.DefaultPlan,
		Settings: make(map[string]interface{}),
		Metadata: make(map[string]interface{}),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Limits:   m.getDefaultLimits(m.config.DefaultPlan),
	}

	// Set trial expiration
	if m.config.TrialDuration > 0 {
		expires := time.Now().Add(m.config.TrialDuration)
		tenant.ExpiresAt = &expires
	}

	// Store tenant
	if err := m.storage.CreateTenant(ctx, tenant); err != nil {
		return nil, fmt.Errorf("failed to create tenant: %w", err)
	}

	// Add owner as tenant user
	if err := m.storage.AddTenantUser(ctx, tenant.ID, ownerID, []string{"owner", "admin"}); err != nil {
		m.logger.Error("Failed to add owner to tenant", "error", err, "tenant_id", tenant.ID, "user_id", ownerID)
	}

	// Update cache
	m.cacheTenant(tenant)

	m.logger.Info("Tenant created", "tenant_id", tenant.ID, "name", name, "owner_id", ownerID)
	return tenant, nil
}

// GetTenant retrieves a tenant by ID
func (m *Manager) GetTenant(ctx context.Context, id string) (*Tenant, error) {
	// Try cache first
	if cached, found := m.cache.Get("tenant:" + id); found {
		if tenant, ok := cached.(*Tenant); ok {
			return tenant, nil
		}
	}

	// Load from storage
	tenant, err := m.storage.GetTenant(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant: %w", err)
	}

	// Update cache
	m.cacheTenant(tenant)

	return tenant, nil
}

// GetTenantByDomain retrieves a tenant by domain
func (m *Manager) GetTenantByDomain(ctx context.Context, domain string) (*Tenant, error) {
	// Try cache first
	if cached, found := m.cache.Get("tenant_domain:" + domain); found {
		if tenant, ok := cached.(*Tenant); ok {
			return tenant, nil
		}
	}

	// Load from storage
	tenant, err := m.storage.GetTenantByDomain(ctx, domain)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant by domain: %w", err)
	}

	// Update cache
	m.cache.Set("tenant_domain:"+domain, tenant, m.config.CacheTTL)

	return tenant, nil
}

// GetTenantBySlug retrieves a tenant by slug
func (m *Manager) GetTenantBySlug(ctx context.Context, slug string) (*Tenant, error) {
	// Try cache first
	if cached, found := m.cache.Get("tenant_slug:" + slug); found {
		if tenant, ok := cached.(*Tenant); ok {
			return tenant, nil
		}
	}

	// Load from storage
	tenant, err := m.storage.GetTenantBySlug(ctx, slug)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant by slug: %w", err)
	}

	// Update cache
	m.cache.Set("tenant_slug:"+slug, tenant, m.config.CacheTTL)

	return tenant, nil
}

// UpdateTenant updates a tenant
func (m *Manager) UpdateTenant(ctx context.Context, tenant *Tenant) error {
	tenant.UpdatedAt = time.Now()

	if err := m.storage.UpdateTenant(ctx, tenant); err != nil {
		return fmt.Errorf("failed to update tenant: %w", err)
	}

	// Update cache
	m.cacheTenant(tenant)

	m.logger.Info("Tenant updated", "tenant_id", tenant.ID, "name", tenant.Name)
	return nil
}

// DeleteTenant deletes a tenant
func (m *Manager) DeleteTenant(ctx context.Context, id string) error {
	if err := m.storage.DeleteTenant(ctx, id); err != nil {
		return fmt.Errorf("failed to delete tenant: %w", err)
	}

	// Clear cache
	m.cache.Delete("tenant:" + id)

	m.logger.Info("Tenant deleted", "tenant_id", id)
	return nil
}

// AddTenantUser adds a user to a tenant
func (m *Manager) AddTenantUser(ctx context.Context, tenantID, userID string, roles []string) error {
	if err := m.storage.AddTenantUser(ctx, tenantID, userID, roles); err != nil {
		return fmt.Errorf("failed to add tenant user: %w", err)
	}

	m.logger.Info("User added to tenant", "tenant_id", tenantID, "user_id", userID, "roles", roles)
	return nil
}

// RemoveTenantUser removes a user from a tenant
func (m *Manager) RemoveTenantUser(ctx context.Context, tenantID, userID string) error {
	if err := m.storage.RemoveTenantUser(ctx, tenantID, userID); err != nil {
		return fmt.Errorf("failed to remove tenant user: %w", err)
	}

	m.logger.Info("User removed from tenant", "tenant_id", tenantID, "user_id", userID)
	return nil
}

// GetTenantUsers returns all users in a tenant
func (m *Manager) GetTenantUsers(ctx context.Context, tenantID string) ([]*TenantUser, error) {
	users, err := m.storage.GetTenantUsers(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get tenant users: %w", err)
	}
	return users, nil
}

// GetUserTenants returns all tenants for a user
func (m *Manager) GetUserTenants(ctx context.Context, userID string) ([]*Tenant, error) {
	tenants, err := m.storage.GetUserTenants(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user tenants: %w", err)
	}
	return tenants, nil
}

// GetTenantContext extracts tenant context from request
func (m *Manager) GetTenantContext(ctx context.Context, domain, slug, userID string) (*TenantContext, error) {
	var tenant *Tenant
	var err error

	// Try to identify tenant by domain first
	if domain != "" && m.config.EnableSubdomains {
		tenant, err = m.GetTenantByDomain(ctx, domain)
		if err != nil {
			// Try slug as fallback
			if slug != "" {
				tenant, err = m.GetTenantBySlug(ctx, slug)
			}
		}
	} else if slug != "" {
		tenant, err = m.GetTenantBySlug(ctx, slug)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to identify tenant: %w", err)
	}

	// Check if tenant is active
	if tenant.Status != TenantStatusActive {
		return nil, fmt.Errorf("tenant is not active (status: %s)", tenant.Status)
	}

	// Check if tenant has expired
	if tenant.ExpiresAt != nil && time.Now().After(*tenant.ExpiresAt) {
		return nil, fmt.Errorf("tenant has expired")
	}

	// Get user roles in tenant
	userRoles := []string{}
	isAdmin := false
	isOwner := false

	if userID != "" {
		tenantUsers, err := m.storage.GetTenantUsers(ctx, tenant.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get tenant users: %w", err)
		}

		for _, tenantUser := range tenantUsers {
			if tenantUser.UserID == userID {
				userRoles = tenantUser.Roles
				for _, role := range userRoles {
					if role == "admin" {
						isAdmin = true
					}
					if role == "owner" {
						isOwner = true
					}
				}
				break
			}
		}

		// If user not found in tenant, deny access
		if len(userRoles) == 0 {
			return nil, fmt.Errorf("user is not a member of this tenant")
		}
	}

	return &TenantContext{
		Tenant:   tenant,
		UserID:   userID,
		IsAdmin:  isAdmin,
		IsOwner:  isOwner,
		Roles:    userRoles,
		Metadata: make(map[string]interface{}),
	}, nil
}

// CheckTenantLimits checks if tenant is within limits
func (m *Manager) CheckTenantLimits(ctx context.Context, tenantID string, limitType string) error {
	tenant, err := m.GetTenant(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("failed to get tenant: %w", err)
	}

	usage, err := m.storage.GetTenantUsage(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("failed to get tenant usage: %w", err)
	}

	switch limitType {
	case "users":
		if tenant.Limits.MaxUsers > 0 && usage.Users >= tenant.Limits.MaxUsers {
			return fmt.Errorf("tenant has reached user limit (%d)", tenant.Limits.MaxUsers)
		}
	case "storage":
		if tenant.Limits.MaxStorage > 0 && usage.StorageUsed >= tenant.Limits.MaxStorage {
			return fmt.Errorf("tenant has reached storage limit (%d MB)", tenant.Limits.MaxStorage)
		}
	case "api_requests":
		if tenant.Limits.MaxAPIRequests > 0 && usage.APIRequests >= tenant.Limits.MaxAPIRequests {
			return fmt.Errorf("tenant has reached API request limit (%d per day)", tenant.Limits.MaxAPIRequests)
		}
	case "connections":
		if tenant.Limits.MaxConnections > 0 && usage.ActiveSessions >= tenant.Limits.MaxConnections {
			return fmt.Errorf("tenant has reached connection limit (%d)", tenant.Limits.MaxConnections)
		}
	}

	return nil
}

// UpdateTenantUsage updates tenant usage statistics
func (m *Manager) UpdateTenantUsage(ctx context.Context, tenantID string, usageType string, delta int) error {
	usage, err := m.storage.GetTenantUsage(ctx, tenantID)
	if err != nil {
		// Create usage record if it doesn't exist
		usage = &TenantUsage{
			TenantID:  tenantID,
			LastReset: time.Now(),
		}
	}

	// Reset daily counters if needed
	now := time.Now()
	if now.Sub(usage.LastReset) >= 24*time.Hour {
		usage.APIRequests = 0
		usage.LastReset = now
	}

	// Update usage based on type
	switch usageType {
	case "users":
		usage.Users += delta
	case "storage":
		usage.StorageUsed += delta
	case "api_requests":
		usage.APIRequests += delta
	case "connections":
		usage.ActiveSessions += delta
	}

	return m.storage.UpdateTenantUsage(ctx, tenantID, usage)
}

// cacheTenant caches a tenant
func (m *Manager) cacheTenant(tenant *Tenant) {
	m.cache.Set("tenant:"+tenant.ID, tenant, m.config.CacheTTL)
	if tenant.Domain != "" {
		m.cache.Set("tenant_domain:"+tenant.Domain, tenant, m.config.CacheTTL)
	}
	m.cache.Set("tenant_slug:"+tenant.Slug, tenant, m.config.CacheTTL)
}

// generateID generates a unique ID
func (m *Manager) generateID() string {
	return fmt.Sprintf("tn_%d", time.Now().UnixNano())
}

// generateSlug generates a slug from name
func (m *Manager) generateSlug(name string) string {
	// Convert to lowercase and replace spaces with hyphens
	slug := strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	
	// Remove special characters
	slug = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return -1
	}, slug)
	
	// Ensure it's not empty
	if slug == "" {
		slug = fmt.Sprintf("tenant-%d", time.Now().Unix())
	}
	
	return slug
}

// getDefaultLimits returns default limits for a plan
func (m *Manager) getDefaultLimits(plan string) *TenantLimits {
	switch plan {
	case "basic":
		return &TenantLimits{
			MaxUsers:       5,
			MaxStorage:     1024,  // 1GB
			MaxAPIRequests: 10000,
			MaxConnections: 10,
		}
	case "pro":
		return &TenantLimits{
			MaxUsers:       50,
			MaxStorage:     10240, // 10GB
			MaxAPIRequests: 100000,
			MaxConnections: 100,
		}
	case "enterprise":
		return &TenantLimits{
			MaxUsers:       -1, // unlimited
			MaxStorage:     -1, // unlimited
			MaxAPIRequests: -1, // unlimited
			MaxConnections: -1, // unlimited
		}
	default:
		return &TenantLimits{
			MaxUsers:       3,
			MaxStorage:     512, // 512MB
			MaxAPIRequests: 1000,
			MaxConnections: 5,
		}
	}
}

// TenantResolver resolves tenant from HTTP requests
type TenantResolver struct {
	manager *Manager
	config  Config
}

// NewTenantResolver creates a new tenant resolver
func NewTenantResolver(manager *Manager, config Config) *TenantResolver {
	return &TenantResolver{
		manager: manager,
		config:  config,
	}
}

// ResolveTenant extracts tenant information from HTTP request
func (tr *TenantResolver) ResolveTenant(ctx context.Context, host, path string) (*Tenant, error) {
	// Extract subdomain if enabled
	if tr.config.EnableSubdomains {
		parts := strings.Split(host, ".")
		if len(parts) > 2 {
			subdomain := parts[0]
			// Skip www subdomain
			if subdomain != "www" {
				tenant, err := tr.manager.GetTenantBySlug(ctx, subdomain)
				if err == nil {
					return tenant, nil
				}
			}
		}
	}

	// Extract from path (e.g., /tenant/{slug}/...)
	pathParts := strings.Split(strings.Trim(path, "/"), "/")
	if len(pathParts) > 0 && pathParts[0] == "tenant" && len(pathParts) > 1 {
		slug := pathParts[1]
		tenant, err := tr.manager.GetTenantBySlug(ctx, slug)
		if err == nil {
			return tenant, nil
		}
	}

	// Return default tenant if configured
	if tr.config.DefaultDomain != "" {
		tenant, err := tr.manager.GetTenantByDomain(ctx, tr.config.DefaultDomain)
		if err == nil {
			return tenant, nil
		}
	}

	return nil, fmt.Errorf("tenant not found")
}
