package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
)

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
				prompt := promptui.Prompt{
					Label:   "Project Name",
					Default: "astra-app",
				}
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
			fmt.Println("Scaffolding Astra application...")
			os.MkdirAll(filepath.Join(name, "app/http/controllers"), 0755)
			os.MkdirAll(filepath.Join(name, "app/models"), 0755)
			os.MkdirAll(filepath.Join(name, "database/migrations"), 0755)
			os.MkdirAll(filepath.Join(name, "config"), 0755)
			os.MkdirAll(filepath.Join(name, "storage/app/public"), 0755)

			envExample := `APP_NAME=` + name + `
APP_ENV=local
APP_KEY=
APP_DEBUG=true
APP_URL=http://localhost:8080

DB_CONNECTION=pgsql
DB_HOST=127.0.0.1
DB_PORT=5432
DB_DATABASE=` + name + `
DB_USERNAME=postgres
DB_PASSWORD=

REDIS_HOST=127.0.0.1
REDIS_PORT=6379
REDIS_PASSWORD=

MAIL_MAILER=log
MAIL_HOST=smtp.mailtrap.io
MAIL_PORT=2525
MAIL_USERNAME=null
MAIL_PASSWORD=null
MAIL_FROM_ADDRESS="hello@example.com"
`
			os.WriteFile(filepath.Join(name, ".env.example"), []byte(envExample), 0644)
			os.WriteFile(filepath.Join(name, ".env"), []byte(envExample), 0644)

			fmt.Println("Astra application created successfully!")
		},
	}

	cmd.Flags().BoolVar(&apiOnly, "api-only", false, "Create an API only application")
	cmd.Flags().BoolVar(&web, "web", false, "Create a web frontend with React + Vite")
	cmd.Flags().BoolVar(&mobile, "mobile", false, "Create a mobile frontend with Expo")

	return cmd
}
