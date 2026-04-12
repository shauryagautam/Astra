package http

import (
	"net/http"
	"sync"

	"github.com/shauryagautam/Astra/pkg/engine"
)

// HealthHandler returns a sophisticated health check handler that runs all registered probes.
// It is fully decoupled from the kernel and accepts an explicit health check registry.
func HealthHandler(checks map[string]engine.HealthProvider) HandlerFunc {
	return func(c *Context) error {
		if len(checks) == 0 {
			return c.JSON(map[string]string{"status": "ok"}, http.StatusOK)
		}

		results := make(map[string]string)
		var mu sync.Mutex
		var wg sync.WaitGroup
		hasError := false

		for name, check := range checks {
			wg.Add(1)
			go func(name string, hp engine.HealthProvider) {
				defer wg.Done()
				err := hp.CheckHealth(c.Ctx())
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					results[name] = err.Error()
					hasError = true
				} else {
					results[name] = "ok"
				}
			}(name, check)
		}

		wg.Wait()

		status := http.StatusOK
		response := map[string]any{
			"status": "ok",
			"checks": results,
		}

		if hasError {
			status = http.StatusServiceUnavailable
			response["status"] = "error"
		}

		return c.JSON(response, status)
	}
}

// ReadyHandler returns a simple liveness probe handler.
func ReadyHandler() HandlerFunc {
	return func(c *Context) error {
		return c.SendString("OK")
	}
}
