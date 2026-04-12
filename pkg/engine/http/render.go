package http

import (
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// ViewEngine defines the interface for template rendering.
type ViewEngine interface {
	Render(wr io.Writer, name string, data any) error
}

// TemplateEngine is the default view engine that wraps html/template
// with layout support, helper functions, and optional auto-reload in dev mode.
type TemplateEngine struct {
	fs        fs.FS
	dir       string
	extension string
	layout    string
	funcMap   template.FuncMap
	isDev     bool

	mu        sync.RWMutex
	templates map[string]*template.Template
}

// TemplateOption is a functional option for configuring the TemplateEngine.
type TemplateOption func(*TemplateEngine)

// WithFS sets a custom filesystem (e.g., embed.FS) for templates.
func WithFS(filesystem fs.FS) TemplateOption {
	return func(e *TemplateEngine) {
		e.fs = filesystem
	}
}

// WithExtension sets the template file extension (default: ".html").
func WithExtension(ext string) TemplateOption {
	return func(e *TemplateEngine) {
		e.extension = ext
	}
}

// WithLayout sets the default layout template name (e.g., "layouts/app").
func WithLayout(layout string) TemplateOption {
	return func(e *TemplateEngine) {
		e.layout = layout
	}
}

// WithFuncMap adds custom template functions.
func WithFuncMap(funcMap template.FuncMap) TemplateOption {
	return func(e *TemplateEngine) {
		for k, v := range funcMap {
			e.funcMap[k] = v
		}
	}
}

// WithDevMode enables auto-reload of templates on every render (no caching).
func WithDevMode(isDev bool) TemplateOption {
	return func(e *TemplateEngine) {
		e.isDev = isDev
	}
}

// NewTemplateEngine creates a new TemplateEngine.
//
// Usage:
//
//	engine := http.NewTemplateEngine("views",
//	    http.WithLayout("layouts/app"),
//	    http.WithDevMode(true),
//	)
//	app.Register("views", engine)
func NewTemplateEngine(dir string, opts ...TemplateOption) *TemplateEngine {
	e := &TemplateEngine{
		dir:       dir,
		extension: ".html",
		funcMap:   defaultFuncMap(),
		templates: make(map[string]*template.Template),
	}

	for _, opt := range opts {
		opt(e)
	}

	return e
}

// Render renders a template by name with the given data and returns the result.
func (e *TemplateEngine) Render(wr io.Writer, name string, data any) error {
	tmpl, err := e.getTemplate(name)
	if err != nil {
		return err
	}

	if err := tmpl.Execute(wr, data); err != nil {
		return fmt.Errorf("views: failed to execute template %q: %w", name, err)
	}
	return nil
}

// getTemplate returns a cached or freshly compiled template.
func (e *TemplateEngine) getTemplate(name string) (*template.Template, error) {
	// In dev mode, always recompile
	if !e.isDev {
		e.mu.RLock()
		if t, ok := e.templates[name]; ok {
			e.mu.RUnlock()
			return t, nil
		}
		e.mu.RUnlock()
	}

	tmpl, err := e.compile(name)
	if err != nil {
		return nil, err
	}

	if !e.isDev {
		e.mu.Lock()
		e.templates[name] = tmpl
		e.mu.Unlock()
	}

	return tmpl, nil
}

// compile parses the template (and optional layout) from the filesystem.
func (e *TemplateEngine) compile(name string) (*template.Template, error) {
	filename := name + e.extension

	var files []string

	// If a layout is set, include it first
	if e.layout != "" {
		layoutFile := e.layout + e.extension
		files = append(files, layoutFile)
	}
	files = append(files, filename)

	// Parse from embedded FS or disk
	if e.fs != nil {
		return template.New(filepath.Base(filename)).Funcs(e.funcMap).ParseFS(e.fs, files...)
	}

	// Parse from disk
	fullPaths := make([]string, len(files))
	for i, f := range files {
		fullPaths[i] = filepath.Join(e.dir, f)
	}
	return template.New(filepath.Base(filename)).Funcs(e.funcMap).ParseFiles(fullPaths...)
}

// Warmup pre-compiles all templates found in the engine's directory.
// Useful for production to avoid late compilation latency.
func (e *TemplateEngine) Warmup() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	return filepath.Walk(e.dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, e.extension) {
			return nil
		}

		rel, err := filepath.Rel(e.dir, path)
		if err != nil {
			return err
		}

		name := strings.TrimSuffix(rel, e.extension)
		tmpl, err := e.compile(name)
		if err != nil {
			return fmt.Errorf("views: failed to warmup %q: %w", name, err)
		}

		e.templates[name] = tmpl
		return nil
	})
}

// defaultFuncMap returns a set of built-in template helper functions.
func defaultFuncMap() template.FuncMap {
	return template.FuncMap{
		// String helpers
		"upper":    strings.ToUpper,
		"lower":    strings.ToLower,
		"title":    cases.Title(language.English).String,
		"contains": strings.Contains,
		"replace":  strings.ReplaceAll,
		"join":     strings.Join,

		// HTML helpers
		"safe": func(s string) template.HTML { // #nosec G203
			return template.HTML(s)
		},
		"safeAttr": func(s string) template.HTMLAttr { // #nosec G203
			return template.HTMLAttr(s)
		},
		"safeURL": func(s string) template.URL { // #nosec G203
			return template.URL(s)
		},

		// Env helper
		"env": func(key string, def ...string) string {
			if val := os.Getenv(key); val != "" {
				return val
			}
			if len(def) > 0 {
				return def[0]
			}
			return ""
		},

		// Dict helper for passing multiple values to templates
		"dict": func(values ...any) map[string]any {
			if len(values)%2 != 0 {
				return nil
			}
			m := make(map[string]any, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					continue
				}
				m[key] = values[i+1]
			}
			return m
		},

		// Conditional helper
		"ternary": func(cond bool, a, b any) any {
			if cond {
				return a
			}
			return b
		},

		// Asset helpers
		"asset_path": func(path string) string {
			return "/static/" + strings.TrimPrefix(path, "/")
		},
		"asset_tag": func(path string) template.HTML {
			ext := filepath.Ext(path)
			fullPath := "/static/" + strings.TrimPrefix(path, "/")
			switch ext {
			case ".css":
				return template.HTML(fmt.Sprintf(`<link rel="stylesheet" href="%s">`, fullPath)) // #nosec G203
			case ".js":
				return template.HTML(fmt.Sprintf(`<script src="%s"></script>`, fullPath)) // #nosec G203
			default:
				return ""
			}
		},

		// CSRF helpers - These now retrieve tokens from the *Context for unified security
		"csrf_token": func(data any) string {
			if m, ok := data.(map[string]any); ok {
				if c, ok := m["Context"].(interface {
					GetString(string) string
				}); ok {
					token := c.GetString("astra_csrf_token")
					if token != "" {
						return token
					}
				}
			}
			panic("CSRF token unavailable. Ensure CSRF middleware is applied and {{ csrf_token . }} is used.")
		},
		"csrf_field": func(data any) template.HTML {
			if m, ok := data.(map[string]any); ok {
				if c, ok := m["Context"].(interface {
					GetString(string) string
				}); ok {
					token := c.GetString("astra_csrf_token")
					if token != "" {
						return template.HTML(fmt.Sprintf(`<input type="hidden" name="_csrf" value="%s">`, token)) // #nosec G203
					}
				}
			}
			panic("CSRF token unavailable. Ensure CSRF middleware is applied and {{ csrf_field . }} is used.")
		},

		// Flash helpers
		"flash": func(data any, name string) string {
			if m, ok := data.(map[string]any); ok {
				if flashes, ok := m["Flashes"].(map[string]string); ok {
					return flashes[name]
				}
			}
			return ""
		},

		// Internationalization
		"T": func(data any, key string, args ...any) string {
			// This expects "Context" to be injected into the data map by Render
			if m, ok := data.(map[string]any); ok {
				if c, ok := m["Context"].(interface {
					T(string, ...any) string
				}); ok {
					return c.T(key, args...)
				}
			}
			return key
		},
		"locale": func(data any) string {
			if m, ok := data.(map[string]any); ok {
				if c, ok := m["Context"].(interface{ Locale() string }); ok {
					return c.Locale()
				}
			}
			return "en"
		},
	}
}

// ─── Context Methods ──────────────────────────────────────────────────

// Render renders an HTML template using the registered view engine and sends
// the response.
//
//	c.Render("pages/home", map[string]any{"title": "Home"})
func (c *Context) Render(name string, data any, status ...int) error {
	code := http.StatusOK
	if len(status) > 0 {
		code = status[0]
	}

	if c.ViewEngine == nil {
		return fmt.Errorf("views: no view engine registered — ensure it is set on context")
	}

	// Auto-inject CSRF token if middleware set it
	if cookie, err := c.Request.Cookie("astra_csrf"); err == nil {
		if m, ok := data.(map[string]any); ok {
			m["CSRFToken"] = cookie.Value
		}
	}

	// Auto-inject Flash messages (and clear them)
	flashes := c.GetFlashes()
	if len(flashes) > 0 {
		if m, ok := data.(map[string]any); ok {
			m["Flashes"] = flashes
		}
		c.ClearFlashes()
	}

	// Auto-inject Context for helpers (T, locale, etc)
	if m, ok := data.(map[string]any); ok {
		m["Context"] = c
	}

	c.Writer.Header().Set("Content-Type", "text/html; charset=utf-8")
	c.Writer.WriteHeader(code)
	c.written = true

	err := c.ViewEngine.Render(c.Writer, name, data)
	if err != nil {
		return err
	}

	return nil
}

