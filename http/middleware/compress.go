package middleware

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"

	astrahttp "github.com/astraframework/astra/http"
)

type gzipWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w gzipWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

// Gzip returns a middleware that compresses HTTP responses using gzip.
func Gzip() astrahttp.MiddlewareFunc {
	return func(next astrahttp.HandlerFunc) astrahttp.HandlerFunc {
		return func(c *astrahttp.Context) error {
			if !strings.Contains(c.Request.Header.Get("Accept-Encoding"), "gzip") {
				return next(c)
			}

			c.Writer.Header().Del("Content-Length")
			c.SetHeader("Content-Encoding", "gzip")
			c.SetHeader("Vary", "Accept-Encoding")
			gz := gzip.NewWriter(c.Writer)
			defer func() {
				if err := gz.Close(); err != nil {
					// Log error but don't fail the request for close errors
					// The response has already been written
				}
			}()

			gzw := gzipWriter{Writer: gz, ResponseWriter: c.Writer}
			c.Writer = gzw

			return next(c)
		}
	}
}
