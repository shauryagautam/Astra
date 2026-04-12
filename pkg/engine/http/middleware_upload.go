package http

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/shauryagautam/Astra/pkg/engine"
)

type UploadMiddleware struct {
	storage engine.Storage
	logger  *slog.Logger
}

func NewUploadMiddleware(s engine.Storage, logger *slog.Logger) *UploadMiddleware {
	return &UploadMiddleware{storage: s, logger: logger}
}

func (m *UploadMiddleware) UploadFile(maxSize int64, allowedTypes []string) MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			c := FromRequest(r)
			if c == nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			if err := r.ParseMultipartForm(maxSize); err != nil {
				c.BadRequestError(fmt.Sprintf("Parse error: %v", err))
				return
			}

			file, header, err := r.FormFile("file")
			if err != nil {
				c.BadRequestError("File missing")
				return
			}
			defer file.Close()

			// Simplified upload logic...
			m.logger.Info("uploading file", "filename", header.Filename)
			
			next.ServeHTTP(w, r)
		})
	}
}
