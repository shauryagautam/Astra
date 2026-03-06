package middleware

import (
	"compress/gzip"
	"io"
	"strings"
	"net/http"

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

			c.SetHeader("Content-Encoding", "gzip")
			gz := gzip.NewWriter(c.Writer)
			defer gz.Close()

			gzw := gzipWriter{Writer: gz, ResponseWriter: c.Writer}
			c.Writer = gzw

			return next(c)
		}
	}
}
