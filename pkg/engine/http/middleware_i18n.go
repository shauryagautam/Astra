package http

import (
	"context"
	"net/http"
	"strings"
)

const (
	ContextLocaleKey = "astra_locale"
	CookieLocaleKey  = "astra_locale"
)

// LocaleMiddleware detects the user locale and stores it in the request context.
func LocaleMiddleware(fallback string) MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			locale := ""

			// 1. Query parameter
			if q := r.URL.Query().Get("lang"); q != "" {
				locale = q
			}

			// 2. Cookie
			if locale == "" {
				if cookie, err := r.Cookie(CookieLocaleKey); err == nil {
					locale = cookie.Value
				}
			}

			// 3. Header Accept-Language
			if locale == "" {
				accept := r.Header.Get("Accept-Language")
				if accept != "" {
					parts := strings.Split(accept, ",")
					if len(parts) > 0 {
						lang := strings.TrimSpace(parts[0])
						if dash := strings.Index(lang, "-"); dash != -1 {
							locale = lang[:dash]
						} else if semi := strings.Index(lang, ";"); semi != -1 {
							locale = lang[:semi]
						} else {
							locale = lang
						}
					}
				}
			}

			// Default fallback
			if locale == "" {
				locale = fallback
			}

			// Store in request context for framework Context to pick up
			ctx := context.WithValue(r.Context(), ContextLocaleKey, locale)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
