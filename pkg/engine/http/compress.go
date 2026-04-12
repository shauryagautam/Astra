package http

import (
	"compress/gzip"
	"io"
	"net/http"
	"strings"

)

type gzipWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w gzipWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

// Gzip returns a standard middleware that compresses HTTP responses using gzip.
func Gzip() MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
				next.ServeHTTP(w, r)
				return
			}

			w.Header().Del("Content-Length")
			w.Header().Set("Content-Encoding", "gzip")
			w.Header().Set("Vary", "Accept-Encoding")
			
			gz, _ := gzip.NewWriterLevel(w, gzip.DefaultCompression)
			defer gz.Close()

			gzw := gzipWriter{Writer: gz, ResponseWriter: w}
			next.ServeHTTP(gzw, r)
		})
	}
}
