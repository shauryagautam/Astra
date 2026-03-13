// Package assets provides an esbuild-based asset pipeline for bundling,
// minifying, and fingerprinting JavaScript and CSS assets.
package assets

import (
	"fmt"
	"github.com/astraframework/astra/json"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// Config holds the asset pipeline configuration.
type Config struct {
	Entrypoints  []string // Source files to bundle (e.g., "resources/js/app.js")
	OutputDir    string   // Output directory (e.g., "public/build")
	PublicPath   string   // URL prefix for assets (e.g., "/build/")
	Minify       bool     // Enable minification (recommended for production)
	Sourcemap    bool     // Generate source maps
	ManifestPath string   // Path to the generated manifest file
}

// Pipeline manages asset building and manifest reading.
type Pipeline struct {
	config   Config
	manifest map[string]string // maps source filenames to hashed output filenames
	mu       sync.RWMutex
}

// New creates a new asset Pipeline.
func New(cfg Config) *Pipeline {
	if cfg.OutputDir == "" {
		cfg.OutputDir = "public/build"
	}
	if cfg.PublicPath == "" {
		cfg.PublicPath = "/build/"
	}
	if cfg.ManifestPath == "" {
		cfg.ManifestPath = filepath.Join(cfg.OutputDir, "manifest.json")
	}
	return &Pipeline{
		config:   cfg,
		manifest: make(map[string]string),
	}
}

// Build runs esbuild to bundle assets for production.
func (p *Pipeline) Build() error {
	args := p.buildArgs()
	cmd := exec.Command("npx", append([]string{"esbuild"}, args...)...) // #nosec G204
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Println("  ⚡ Building assets...")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("assets: esbuild failed: %w", err)
	}

	// Generate manifest from output
	if err := p.generateManifest(); err != nil {
		return fmt.Errorf("assets: failed to generate manifest: %w", err)
	}

	fmt.Println("  ✓ Assets built successfully")
	return nil
}

// Watch runs esbuild in watch mode for development.
func (p *Pipeline) Watch() (*exec.Cmd, error) {
	args := append(p.buildArgs(), "--watch")
	cmd := exec.Command("npx", append([]string{"esbuild"}, args...)...) // #nosec G204
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	fmt.Println("  👁 Watching assets for changes...")
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("assets: esbuild watch failed: %w", err)
	}

	return cmd, nil
}

func (p *Pipeline) buildArgs() []string {
	args := make([]string, 0, 16)
	args = append(args, p.config.Entrypoints...)
	args = append(args, "--bundle")
	args = append(args, "--outdir="+p.config.OutputDir)

	if p.config.Minify {
		args = append(args, "--minify")
	}
	if p.config.Sourcemap {
		args = append(args, "--sourcemap")
	}

	// Fingerprint output files for cache busting
	args = append(args, "--entry-names=[name]-[hash]")
	args = append(args, "--asset-names=[name]-[hash]")

	return args
}

// generateManifest reads the esbuild output directory and creates a JSON
// mapping from original filenames to their hashed counterparts.
func (p *Pipeline) generateManifest() error {
	manifest := make(map[string]string)

	entries, err := os.ReadDir(p.config.OutputDir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		// Skip the manifest file itself and source maps
		if name == "manifest.json" || filepath.Ext(name) == ".map" {
			continue
		}
		manifest[name] = p.config.PublicPath + name
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(p.config.ManifestPath, data, 0600); err != nil {
		return err
	}

	p.mu.Lock()
	p.manifest = manifest
	p.mu.Unlock()

	return nil
}

// AssetPath resolves a source filename to its hashed URL using the manifest.
// Returns the original path if no manifest entry is found.
//
//	pipeline.AssetPath("app.js") → "/build/app-abc123.js"
func (p *Pipeline) AssetPath(name string) string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if url, ok := p.manifest[name]; ok {
		return url
	}
	return p.config.PublicPath + name
}

// LoadManifest reads and caches the manifest file from disk.
// Call this at application startup in production.
func (p *Pipeline) LoadManifest() error {
	data, err := os.ReadFile(p.config.ManifestPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No manifest yet, not an error
		}
		return err
	}

	manifest := make(map[string]string)
	if err := json.Unmarshal(data, &manifest); err != nil {
		return err
	}

	p.mu.Lock()
	p.manifest = manifest
	p.mu.Unlock()

	return nil
}

// TemplateHelpers returns template function map entries for asset URL resolution.
// Use with http.NewTemplateEngine:
//
//	engine := http.NewTemplateEngine("views", http.WithFuncMap(pipeline.TemplateHelpers()))
//
// Then in templates:
//
//	<script src="{{ asset "app.js" }}"></script>
func (p *Pipeline) TemplateHelpers() map[string]any {
	return map[string]any{
		"asset": p.AssetPath,
	}
}
