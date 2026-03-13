package i18n

import (
	"strings"

	"github.com/astraframework/astra/http"
)

const (
	ContextLocaleKey = "astra_locale"
	CookieLocaleKey  = "astra_locale"
)

// LocaleMiddleware detects the user locale and stores it in the context.
func LocaleMiddleware(fallback string) http.MiddlewareFunc {
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(c *http.Context) error {
			locale := ""

			// 1. Query parameter
			if q := c.Query("lang"); q != "" {
				locale = q
			}

			// 2. Cookie
			if locale == "" {
				if cookie, err := c.Request.Cookie(CookieLocaleKey); err == nil {
					locale = cookie.Value
				}
			}

			// 3. Header Accept-Language
			if locale == "" {
				accept := c.Header("Accept-Language")
				if accept != "" {
					// Extract first part: en-US,en;q=0.9 -> en
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

			c.Set(ContextLocaleKey, locale)
			return next(c)
		}
	}
}
