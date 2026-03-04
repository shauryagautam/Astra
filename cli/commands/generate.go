package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

func GenerateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate [type] [name]",
		Short: "Generate boilerplate code",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			genType := args[0]
			name := args[1]

			fmt.Printf("Generating %s: %s\n", genType, name)
			
			switch genType {
			case "model":
				generateFile("models", name, "type "+strings.Title(name)+" struct {\n\tastra.Model\n}\n")
			case "controller":
				generateFile("controllers", name, "type "+strings.Title(name)+"Controller struct {}\n")
			default:
				fmt.Println("Unknown generator type")
			}
		},
	}
	return cmd
}

func generateFile(dir, name, content string) {
	os.MkdirAll(dir, 0755)
	os.WriteFile(fmt.Sprintf("%s/%s.go", dir, strings.ToLower(name)), []byte("package "+dir+"\n\n"+content), 0644)
}
