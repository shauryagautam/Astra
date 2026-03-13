package commands

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

type NewTemplateData struct {
	AppName        string
	AppKey         string
	JWTSecret      string
	SecurityConfig string
}

func generateRandomKey(length int) string {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "static_fallback_key_32_chars_min"
	}
	return base64.StdEncoding.EncodeToString(b)
}

func NewCmd() *cobra.Command {
	var apiOnly, web, mobile bool

	cmd := &cobra.Command{
		Use:   "new [name]",
		Short: "Create a new Astra application",
		Run: func(cmd *cobra.Command, args []string) {
			name := ""
			if len(args) > 0 {
				name = args[0]
			} else {
				prompt := promptui.Prompt{Label: "Project Name", Default: "astra-app"}
				var err error
				name, err = prompt.Run()
				if err != nil {
					fmt.Printf("Prompt failed %v\n", err)
					return
				}
			}

			if !apiOnly && !web && !mobile {
				prompt := promptui.Select{
					Label: "Select Project Type",
					Items: []string{"API Only", "Web (React + Vite)", "Mobile (Expo)", "Full Stack (Web + Mobile)"},
				}
				_, result, err := prompt.Run()
				if err != nil {
					fmt.Printf("Prompt failed %v\n", err)
					return
				}
				switch result {
				case "API Only":
					apiOnly = true
				case "Web (React + Vite)":
					web = true
				case "Mobile (Expo)":
					mobile = true
				case "Full Stack (Web + Mobile)":
					web = true
					mobile = true
				}
			}

			data := NewTemplateData{
				AppName:        name,
				AppKey:         generateRandomKey(32),
				JWTSecret:      generateRandomKey(32),
				SecurityConfig: "http.DefaultSSRSecurityConfig()",
			}
			if apiOnly {
				data.SecurityConfig = "http.DefaultAPISecurityConfig()"
			}

			fmt.Printf("Scaffolding Astra application %q...\n", name)

			// Standard Directories
			dirs := []string{
				"app/controllers",
				"app/models",
				"app/jobs",
				"database/migrations",
				"database/seeders",
				"shared/astra-client",
			}
			for _, d := range dirs {
				writeGitKeep(filepath.Join(name, d))
			}

			// Generate Files from Stubs
			files := map[string]string{
				"main.go":            "stubs/main.go.tmpl",
				"routes/api.go":      "stubs/routes.api.go.tmpl",
				".env":               "stubs/env.tmpl",
				".env.example":       "stubs/env.tmpl",
				".gitignore":         "stubs/gitignore.tmpl",
				"Dockerfile":         "stubs/Dockerfile.tmpl",
				"docker-compose.yml": "stubs/docker-compose.yml.tmpl",
				"Makefile":           "stubs/makefile.tmpl",
				"fly.toml":           "stubs/fly.toml.tmpl",
				"README.md":          "stubs/readme.tmpl",
			}

			for target, stub := range files {
				if err := executeNewStub(stub, filepath.Join(name, target), data); err != nil {
					fmt.Printf("  ✗ Failed to create %s: %v\n", target, err)
				} else {
					fmt.Printf("  ✓ Created %s\n", target)
				}
			}

			// Special: go.mod (simple enough for string)
			writeFile(filepath.Join(name, "go.mod"), fmt.Sprintf("module %s\n\ngo 1.22\n", name))

			// Scaffolding conditionals
			if web {
				writeWebPackageJSON(name)
			}

			if mobile {
				writeMobilePackageJSON(name)
			}

			fmt.Println("\nAstra application created successfully!")
			fmt.Printf("To get started:\n  cd %s\n  make setup\n  astra dev\n", name)
		},
	}

	cmd.Flags().BoolVar(&apiOnly, "api-only", false, "Create an API only application")
	cmd.Flags().BoolVar(&web, "web", false, "Create a web frontend with React + Vite")
	cmd.Flags().BoolVar(&mobile, "mobile", false, "Create a mobile frontend with Expo")

	return cmd
}

func executeNewStub(stubPath, outputPath string, data NewTemplateData) error {
	content, err := stubFS.ReadFile(stubPath)
	if err != nil {
		return err
	}

	tmpl, err := template.New(filepath.Base(stubPath)).Parse(string(content))
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(outputPath), 0750); err != nil {
		return err
	}

	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	return tmpl.Execute(file, data)
}

func writeWebPackageJSON(name string) {
	content := `{
  "name": "web",
  "private": true,
  "version": "0.0.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc && vite build"
  },
  "dependencies": {
    "react": "^18.2.0",
    "react-dom": "^18.2.0",
    "@tanstack/react-query": "^5.0.0",
    "astra-client": "file:../shared/astra-client"
  },
  "devDependencies": {
    "@vitejs/plugin-react": "^4.2.1",
    "typescript": "^5.2.2",
    "vite": "^5.1.4"
  }
}`
	writeFile(filepath.Join(name, "web", "package.json"), content)
}

func writeMobilePackageJSON(name string) {
	content := `{
  "name": "mobile",
  "version": "1.0.0",
  "main": "node_modules/expo/AppEntry.js",
  "scripts": {
    "start": "expo start",
    "android": "expo start --android",
    "ios": "expo start --ios"
  },
  "dependencies": {
    "expo": "~50.0.0",
    "expo-secure-store": "~12.8.1",
    "react": "18.2.0",
    "react-native": "0.73.4",
    "astra-client": "file:../shared/astra-client"
  }
}`
	writeFile(filepath.Join(name, "mobile", "package.json"), content)
	writeFile(filepath.Join(name, "mobile", "lib", "auth.ts"), `
import * as SecureStore from 'expo-secure-store';

export const tokenStorage = {
  get: () => SecureStore.getItemAsync('token'),
  set: (val: string) => SecureStore.setItemAsync('token', val),
  remove: () => SecureStore.deleteItemAsync('token')
};
`)
}

func writeFile(path string, content string) {
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		return
	}
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0644); err != nil {
		return
	}
}

func writeGitKeep(dir string) {
	if err := os.MkdirAll(dir, 0750); err != nil {
		return
	}
	if err := os.WriteFile(filepath.Join(dir, ".gitkeep"), []byte(""), 0644); err != nil {
		return
	}
}
