package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/devtdq1701/jk/pkg/config"
)

var (
	ctxURL   string
	ctxUser  string
	ctxToken string

	contextCmd = &cobra.Command{
		Use:   "context",
		Short: "Manage Jenkins controller contexts",
	}

	contextAddCmd = &cobra.Command{
		Use:   "add <name>",
		Short: "Add or update a controller context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			if ctxURL == "" {
				return fmt.Errorf("--url is required")
			}
			if ctxUser == "" {
				return fmt.Errorf("--user is required")
			}
			if ctxToken == "" {
				return fmt.Errorf("--token is required")
			}

			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}

			cfg.Contexts[name] = config.Context{
				URL:      strings.TrimRight(ctxURL, "/"),
				Username: ctxUser,
				Token:    ctxToken,
			}

			if cfg.CurrentContext == "" {
				cfg.CurrentContext = name
			}

			if err := cfg.Save(cfgPath); err != nil {
				return err
			}

			fmt.Printf("Context '%s' saved.\n", name)
			return nil
		},
	}

	contextLsCmd = &cobra.Command{
		Use:   "ls",
		Short: "List all configured contexts",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}

			if len(cfg.Contexts) == 0 {
				fmt.Println("No contexts found. Add one with 'jk context add <name> --url <url> --user <user> --token <token>'")
				return nil
			}

			for name, c := range cfg.Contexts {
				prefix := "  "
				if name == cfg.CurrentContext {
					prefix = "* "
				}
				fmt.Printf("%s%-15s %s (%s)\n", prefix, name, c.URL, c.Username)
			}
			return nil
		},
	}

	contextCurrentCmd = &cobra.Command{
		Use:   "current",
		Short: "Display the active context name",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}
			if cfg.CurrentContext == "" {
				fmt.Println("No active context")
				return nil
			}
			fmt.Println(cfg.CurrentContext)
			return nil
		},
	}

	contextUseCmd = &cobra.Command{
		Use:   "use <name>",
		Short: "Set the active context",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			cfg, err := config.Load(cfgPath)
			if err != nil {
				return err
			}

			if _, ok := cfg.Contexts[name]; !ok {
				return fmt.Errorf("context '%s' not found", name)
			}

			cfg.CurrentContext = name
			if err := cfg.Save(cfgPath); err != nil {
				return err
			}

			fmt.Printf("Switched to context '%s'.\n", name)
			return nil
		},
	}
)

func init() {
	contextAddCmd.Flags().StringVar(&ctxURL, "url", "", "Jenkins base URL")
	contextAddCmd.Flags().StringVar(&ctxUser, "user", "", "Jenkins username")
	contextAddCmd.Flags().StringVar(&ctxToken, "token", "", "Jenkins API token")

	contextCmd.AddCommand(contextAddCmd)
	contextCmd.AddCommand(contextLsCmd)
	contextCmd.AddCommand(contextCurrentCmd)
	contextCmd.AddCommand(contextUseCmd)

	rootCmd.AddCommand(contextCmd)
}
