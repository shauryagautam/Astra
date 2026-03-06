package http

import (
	"bytes"
	"fmt"
	"html/template"
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
	Render(name string, data any) (string, error)
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
func (e *TemplateEngine) Render(name string, data any) (string, error) {
	tmpl, err := e.getTemplate(name)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("views: failed to execute template %q: %w", name, err)
	}
	return buf.String(), nil
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
		"safe": func(s string) template.HTML {
			return template.HTML(s)
		},
		"safeAttr": func(s string) template.HTMLAttr {
			return template.HTMLAttr(s)
		},
		"safeURL": func(s string) template.URL {
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
	}
}

// ─── Context Methods ──────────────────────────────────────────────────

// Render renders an HTML template using the registered view engine and sends
// the response. Requires a ViewEngine registered as "views" in the App.
//
//	c.Render("pages/home", map[string]any{"title": "Home"})
func (c *Context) Render(name string, data any, status ...int) error {
	code := http.StatusOK
	if len(status) > 0 {
		code = status[0]
	}

	if c.App == nil {
		return fmt.Errorf("views: no App registered on context")
	}

	engine := c.App.Get("views")
	if engine == nil {
		return fmt.Errorf("views: no view engine registered — register a ViewEngine as 'views'")
	}

	ve, ok := engine.(ViewEngine)
	if !ok {
		return fmt.Errorf("views: registered 'views' service does not implement ViewEngine")
	}

	html, err := ve.Render(name, data)
	if err != nil {
		return err
	}

	return c.HTML(html, code)
}


