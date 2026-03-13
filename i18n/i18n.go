package i18n

import (
	"fmt"
	"github.com/astraframework/astra/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Translator is the interface for translating keys.
type Translator interface {
	T(locale, key string, args ...any) string
	Has(locale, key string) bool
}

// Engine implements Translator and handles translation loading.
type Engine struct {
	mu           sync.RWMutex
	translations map[string]map[string]string // locale -> key -> discovery
	fallback     string
}

// NewEngine creates a new translation engine.
func NewEngine(fallback string) *Engine {
	if fallback == "" {
		fallback = "en"
	}
	return &Engine{
		translations: make(map[string]map[string]string),
		fallback:     fallback,
	}
}

// Load loads translations from a directory.
// Files should be named {locale}.json.
func (e *Engine) Load(dir string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	matches, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return err
	}

	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		locale := strings.TrimSuffix(filepath.Base(path), ".json")
		var m map[string]string
		if err := json.Unmarshal(data, &m); err != nil {
			return fmt.Errorf("i18n: failed to parse %s: %w", path, err)
		}

		e.translations[locale] = m
	}

	return nil
}

// T translates a key in the given locale.
func (e *Engine) T(locale, key string, args ...any) string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// 1. Try requested locale
	if trans, ok := e.translations[locale]; ok {
		if val, ok := trans[key]; ok {
			return e.format(val, args...)
		}
	}

	// 2. Try fallback locale
	if locale != e.fallback {
		if trans, ok := e.translations[e.fallback]; ok {
			if val, ok := trans[key]; ok {
				return e.format(val, args...)
			}
		}
	}

	// 3. Return key itself if not found
	return key
}

// Has checks if a translation key exists.
func (e *Engine) Has(locale, key string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if trans, ok := e.translations[locale]; ok {
		if _, ok := trans[key]; ok {
			return true
		}
	}
	return false
}

func (e *Engine) format(s string, args ...any) string {
	if len(args) == 0 {
		return s
	}

	// Basic placeholder replacement: {0}, {1} or {name} if args is a map
	// For simplicity, let's support positional args via fmt.Sprintf-like logic if it contains %
	if strings.Contains(s, "%") {
		return fmt.Sprintf(s, args...)
	}

	// Simple positional replacement if strings contains {i}
	for i, arg := range args {
		placeholder := fmt.Sprintf("{%d}", i)
		s = strings.ReplaceAll(s, placeholder, fmt.Sprint(arg))
	}

	// Map-based replacement if args[0] is a map
	if len(args) == 1 {
		if m, ok := args[0].(map[string]any); ok {
			for k, v := range m {
				placeholder := fmt.Sprintf("{%s}", k)
				s = strings.ReplaceAll(s, placeholder, fmt.Sprint(v))
			}
		}
	}

	return s
}
