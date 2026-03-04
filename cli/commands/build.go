package commands

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

func BuildCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "build",
		Short: "Build the application for production",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Building Astra application...")

			c := exec.Command("go", "build", "-o", "bin/app", "main.go")
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			
			if err := c.Run(); err != nil {
				fmt.Println("Build failed:", err)
			} else {
				fmt.Println("Build successful: bin/app")
			}
		},
	}
}
