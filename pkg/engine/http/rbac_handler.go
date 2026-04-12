package http

import (

	"log/slog"
	stdhttp "net/http"

	"github.com/shauryagautam/Astra/pkg/identity/rbac"
)

// RBACHandler provides RBAC HTTP handlers
type RBACHandler struct {
	rbac   *rbac.RBAC
	logger *slog.Logger
}

// NewRBACHandler creates new RBAC handler
func NewRBACHandler(r *rbac.RBAC, logger *slog.Logger) *RBACHandler {
	return &RBACHandler{
		rbac:   r,
		logger: logger,
	}
}

// CheckAccess handles access check requests
func (h *RBACHandler) CheckAccess(c *Context) error {
	var req rbac.AccessRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(map[string]string{"error": "Invalid request format"}, stdhttp.StatusBadRequest)
	}

	result, err := h.rbac.CheckAccess(c.Request.Context(), &req)
	if err != nil {
		h.logger.Error("Access check failed", "error", err)
		return c.JSON(map[string]string{"error": "Access check failed"}, stdhttp.StatusInternalServerError)
	}

	return c.JSON(result, stdhttp.StatusOK)
}

// ServeHTTP makes RBACHandler a standard net/http handler
func (h *RBACHandler) ServeHTTP(w stdhttp.ResponseWriter, r *stdhttp.Request) {
	c := NewContext(w, r) 
	defer c.release()

	if err := h.CheckAccess(c); err != nil {
		h.logger.Error("HTTP handler error", "error", err)
	}
}
