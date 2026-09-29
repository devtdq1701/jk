package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var stagesCmd = &cobra.Command{
	Use:   "stages <job> [build_no]",
	Short: "List pipeline stages and their execution status",
	Args:  cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := getClient()
		if err != nil {
			return err
		}

		job := args[0]
		buildNo := ""
		if len(args) > 1 {
			buildNo = args[1]
		}

		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		stages, err := client.GetStages(ctx, job, buildNo)
		if err != nil {
			return err
		}

		if len(stages) == 0 {
			fmt.Println("No stages found.")
			return nil
		}

		fmt.Printf("%-6s %-25s %-12s %s\n", "ID", "STAGE", "STATUS", "DURATION")
		for _, s := range stages {
			dur := (time.Duration(s.DurationMillis) * time.Millisecond).Round(time.Second)
			fmt.Printf("%-6s %-25s %-12s %s\n", s.ID, s.Name, s.Status, dur)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(stagesCmd)
}
