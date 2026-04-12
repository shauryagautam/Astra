package http

import (
	"encoding/gob"
)

func init() {
	// Register map for flash messages in gob for session storage
	gob.Register(map[string]string{})
}

const flashSessionKey = "astra_flash"

// Flash adds a flash message to the session.
func (c *Context) Flash(key, message string) {
	sess := c.Session()
	if sess == nil {
		return
	}

	flashes := c.GetFlashes()
	flashes[key] = message
	sess.Set(flashSessionKey, flashes)
}

// GetFlashes retrieves all flash messages and clears them from the session.
func (c *Context) GetFlashes() map[string]string {
	sess := c.Session()
	if sess == nil {
		return make(map[string]string)
	}

	val := sess.Get(flashSessionKey)
	if val == nil {
		return make(map[string]string)
	}

	flashes, ok := val.(map[string]string)
	if !ok {
		return make(map[string]string)
	}

	return flashes
}

// ClearFlashes clears flash messages from the session.
func (c *Context) ClearFlashes() {
	sess := c.Session()
	if sess != nil {
		sess.Delete(flashSessionKey)
	}
}

// ErrorFlash is a convenience method for flashing error messages.
func (c *Context) ErrorFlash(message string) {
	c.Flash("error", message)
}

// SuccessFlash is a convenience method for flashing success messages.
func (c *Context) SuccessFlash(message string) {
	c.Flash("success", message)
}
