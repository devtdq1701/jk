package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	followLogFlag bool
	stageLogFlag  string

	logCmd = &cobra.Command{
		Use:   "log <job> [build_no]",
		Short: "View console log or stage log of a job build",
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

			if stageLogFlag != "" {
				ctx, cancel := context.WithTimeout(context.Background(), timeout)
				defer cancel()

				stageLog, err := client.GetStageLog(ctx, job, buildNo, stageLogFlag)
				if err != nil {
					return err
				}
				fmt.Print(stageLog)
				return nil
			}

			// Console log (follow or snapshot)
			ctx := context.Background()
			return client.StreamLog(ctx, job, buildNo, followLogFlag, os.Stdout)
		},
	}
)

func init() {
	logCmd.Flags().BoolVarP(&followLogFlag, "follow", "f", false, "follow log stream until completion")
	logCmd.Flags().StringVar(&stageLogFlag, "stage", "", "stage name or ID to view log for")

	rootCmd.AddCommand(logCmd)
}
