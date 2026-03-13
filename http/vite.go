package http

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
)

// ViteManifest represents the structure of a Vite manifest.json file.
type ViteManifest map[string]struct {
	File           string   `json:"file"`
	CSS            []string `json:"css"`
	Assets         []string `json:"assets"`
	IsEntry        bool     `json:"isEntry"`
	Src            string   `json:"src"`
	IsDynamicEntry bool     `json:"isDynamicEntry"`
}

type ManifestManager struct {
	mu       sync.RWMutex
	manifest ViteManifest
	path     string
	dev      bool
}

func NewManifestManager(path string, isDev bool) *ManifestManager {
	return &ManifestManager{
		path: path,
		dev:  isDev,
	}
}

func (m *ManifestManager) Load() error {
	if m.dev {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	data, err := os.ReadFile(m.path)
	if err != nil {
		return fmt.Errorf("failed to read vite manifest: %w", err)
	}

	if err := json.Unmarshal(data, &m.manifest); err != nil {
		return fmt.Errorf("failed to parse vite manifest: %w", err)
	}

	return nil
}

func (m *ManifestManager) GetEntryFile(entry string) string {
	if m.dev {
		return "/@vite/client" // Should be handled by Vite dev server
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if item, ok := m.manifest[entry]; ok {
		return "/assets/" + item.File
	}
	return ""
}

func (m *ManifestManager) GetCSSFiles(entry string) []string {
	if m.dev {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	if item, ok := m.manifest[entry]; ok {
		var files []string
		for _, css := range item.CSS {
			files = append(files, "/assets/"+css)
		}
		return files
	}
	return nil
}

// UseViteManifest registers the "vite" template function with the router.
func (r *Router) UseViteManifest(path string, isDev bool) {
	mm := NewManifestManager(path, isDev)
	_ = mm.Load() // Lazy load or on startup

	r.AddFunc("vite", func(entry string) string {
		if isDev {
			return fmt.Sprintf(`<script type="module" src="http://localhost:5173/@vite/client"></script><script type="module" src="http://localhost:5173/%s"></script>`, entry)
		}

		file := mm.GetEntryFile(entry)
		cssFiles := mm.GetCSSFiles(entry)

		html := ""
		for _, css := range cssFiles {
			html += fmt.Sprintf(`<link rel="stylesheet" href="%s">`, css)
		}
		html += fmt.Sprintf(`<script type="module" src="%s"></script>`, file)
		return html
	})
}
