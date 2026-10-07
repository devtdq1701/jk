package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

var (
	configFileFlag string
	stdinFlag      bool

	jobCmd = &cobra.Command{
		Use:   "job",
		Short: "Inspect and manage Jenkins jobs",
	}

	jobLsCmd = &cobra.Command{
		Use:   "ls [folder]",
		Short: "List jobs in root or inside a folder",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := getClient()
			if err != nil {
				return err
			}

			folder := ""
			if len(args) > 0 {
				folder = args[0]
			}

			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			jobs, err := client.ListJobs(ctx, folder)
			if err != nil {
				return err
			}

			if len(jobs) == 0 {
				fmt.Println("No jobs found.")
				return nil
			}

			for _, j := range jobs {
				status := parseColor(j.Color)
				fmt.Printf("%-12s %s\n", status, j.Name)
			}
			return nil
		},
	}

	jobGetConfigCmd = &cobra.Command{
		Use:   "get-config <job>",
		Short: "Get config.xml of a job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := getClient()
			if err != nil {
				return err
			}

			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			xmlContent, err := client.GetJobConfig(ctx, args[0])
			if err != nil {
				return err
			}

			fmt.Print(xmlContent)
			return nil
		},
	}

	jobSetConfigCmd = &cobra.Command{
		Use:   "set-config <job>",
		Short: "Update config.xml of a job from file or stdin",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := getClient()
			if err != nil {
				return err
			}

			var data []byte
			if stdinFlag {
				var err error
				data, err = io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("read stdin: %w", err)
				}
			} else if configFileFlag != "" {
				var err error
				data, err = os.ReadFile(configFileFlag)
				if err != nil {
					return fmt.Errorf("read file '%s': %w", configFileFlag, err)
				}
			} else {
				return fmt.Errorf("must specify either -f <file> or --stdin")
			}

			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()

			if err := client.SetJobConfig(ctx, args[0], data); err != nil {
				return err
			}

			fmt.Printf("Config for job '%s' updated successfully.\n", args[0])
			return nil
		},
	}
)

func parseColor(c string) string {
	switch {
	case strings.HasPrefix(c, "blue"):
		return "[SUCCESS]"
	case strings.HasPrefix(c, "red"):
		return "[FAILED]"
	case strings.HasPrefix(c, "yellow"):
		return "[UNSTABLE]"
	case strings.HasSuffix(c, "_anime"):
		return "[BUILDING]"
	case c == "disabled":
		return "[DISABLED]"
	case c == "notbuilt":
		return "[NOT_BUILT]"
	default:
		return "[" + strings.ToUpper(c) + "]"
	}
}

func init() {
	jobSetConfigCmd.Flags().StringVarP(&configFileFlag, "file", "f", "", "path to XML config file")
	jobSetConfigCmd.Flags().BoolVar(&stdinFlag, "stdin", false, "read XML config from stdin")

	jobCmd.AddCommand(jobLsCmd)
	jobCmd.AddCommand(jobGetConfigCmd)
	jobCmd.AddCommand(jobSetConfigCmd)

	rootCmd.AddCommand(jobCmd)
}
