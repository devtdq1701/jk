package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check controller connectivity, authentication, and crumb readiness",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getClient()
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		res, err := client.Check(ctx)
		if err != nil {
			return fmt.Errorf("[FAIL] %w", err)
		}

		fmt.Printf("[OK] Connect:   %s (%dms)\n", res.URL, res.Latency/time.Millisecond)
		if res.User != "" {
			fmt.Printf("[OK] Auth:      Logged in as '%s'\n", res.User)
		} else {
			fmt.Printf("[OK] Auth:      Connected\n")
		}

		if res.CSRFEnabled {
			fmt.Printf("[OK] CSRF:      Crumb issuer available (%s)\n", res.CrumbField)
		} else {
			fmt.Printf("[--] CSRF:      Not enabled on controller\n")
		}

		fmt.Println("Context is healthy and ready.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(checkCmd)
}
