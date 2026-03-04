package commands

import (
	"fmt"
	"github.com/astraframework/astra/codegen"
	"github.com/spf13/cobra"
)

func ClientCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "client",
		Short: "Generate TypeScript client",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Generating TypeScript client...")
			err := codegen.GenerateClient(".", "shared/astra-client")
			if err != nil {
				fmt.Println("Failed to generate client:", err)
			} else {
				fmt.Println("TypeScript client generated in shared/astra-client")
			}
		},
	}
}
