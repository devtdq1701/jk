package cmd

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

var (
	credCmd = &cobra.Command{
		Use:   "cred",
		Short: "Inspect Jenkins credentials",
	}

	credLsCmd = &cobra.Command{
		Use:   "ls",
		Short: "List system credentials (id, type, description)",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := getClient()
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			creds, err := client.ListCredentials(ctx)
			if err != nil {
				return err
			}

			if len(creds) == 0 {
				fmt.Println("No credentials found.")
				return nil
			}

			fmt.Printf("%-30s %-35s %s\n", "ID", "TYPE", "DESCRIPTION")
			for _, c := range creds {
				desc := c.Description
				if desc == "" {
					desc = "-"
				}
				fmt.Printf("%-30s %-35s %s\n", c.ID, c.TypeName, desc)
			}
			return nil
		},
	}
)

func init() {
	credCmd.AddCommand(credLsCmd)
	rootCmd.AddCommand(credCmd)
}
