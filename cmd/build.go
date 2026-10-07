package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var (
	buildParamFlag []string
	buildLimitFlag int
	buildJSONFlag  bool

	buildCmd = &cobra.Command{
		Use:   "build <job> | build ls <job> | build info <job> [buildNo]",
		Short: "Trigger a job build, or inspect build history and details",
		Long: `Manage and inspect Jenkins builds.

Usage:
  jk build <job> [-p key=val]         Trigger a new build (supports parameters)
  jk build ls <job> [-n 10] [--json]  List recent builds with status and duration
  jk build info <job> [buildNo]       Show detailed build info, parameters, and causes`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return cmd.Help()
			}
			switch args[0] {
			case "ls", "list":
				if len(args) < 2 {
					return fmt.Errorf("job name required: jk build ls <job>")
				}
				return runBuildLs(args[1])
			case "info", "show":
				if len(args) < 2 {
					return fmt.Errorf("job name required: jk build info <job> [buildNo]")
				}
				buildNo := "last"
				if len(args) >= 3 {
					buildNo = args[2]
				}
				return runBuildInfo(args[1], buildNo)
			default:
				return runBuildTrigger(args[0])
			}
		},
	}

	buildLsCmd = &cobra.Command{
		Use:   "ls <job>",
		Short: "List recent builds with status and duration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runBuildLs(args[0])
		},
	}

	buildInfoCmd = &cobra.Command{
		Use:   "info <job> [buildNo]",
		Short: "Show detailed build info, parameters, and causes",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			buildNo := "last"
			if len(args) == 2 {
				buildNo = args[1]
			}
			return runBuildInfo(args[0], buildNo)
		},
	}
)

func runBuildTrigger(jobName string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	params := make(map[string]string)
	for _, p := range buildParamFlag {
		parts := strings.SplitN(p, "=", 2)
		if len(parts) == 2 {
			params[parts[0]] = parts[1]
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	loc, err := client.BuildJob(ctx, jobName, params)
	if err != nil {
		return err
	}

	if loc != "" {
		fmt.Printf("Build queued at: %s\n", loc)
	} else {
		fmt.Printf("Build triggered for job '%s'.\n", jobName)
	}
	return nil
}

func runBuildLs(jobName string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	builds, err := client.GetBuilds(ctx, jobName, buildLimitFlag)
	if err != nil {
		return err
	}

	if buildJSONFlag {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(builds)
	}

	if len(builds) == 0 {
		fmt.Println("No builds found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "BUILD\tSTATUS\tSTARTED\tDURATION")
	for _, b := range builds {
		status := "[" + b.Result + "]"
		if b.Result == "" {
			status = "[BUILDING]"
		}

		startTime := "-"
		if b.Timestamp > 0 {
			t := time.UnixMilli(b.Timestamp)
			startTime = t.Format("2006-01-02 15:04:05")
		}

		durationStr := "-"
		if b.Duration > 0 {
			d := time.Duration(b.Duration) * time.Millisecond
			durationStr = formatDuration(d)
		}

		fmt.Fprintf(w, "#%d\t%-12s\t%s\t%s\n", b.Number, status, startTime, durationStr)
	}
	return w.Flush()
}

func runBuildInfo(jobName, buildNo string) error {
	client, err := getClient()
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	info, err := client.GetBuildInfo(ctx, jobName, buildNo)
	if err != nil {
		return err
	}

	if buildJSONFlag {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(info)
	}

	status := "[" + info.Result + "]"
	if info.Building || info.Result == "" {
		status = "[BUILDING]"
	}

	startTime := "-"
	timeAgo := ""
	if info.Timestamp > 0 {
		t := time.UnixMilli(info.Timestamp)
		startTime = t.Format("2006-01-02 15:04:05")
		timeAgo = fmt.Sprintf(" (%s ago)", timeSinceCompact(t))
	}

	durationStr := "-"
	if info.Duration > 0 {
		d := time.Duration(info.Duration) * time.Millisecond
		durationStr = formatDuration(d)
	}

	fmt.Printf("Job:        %s\n", jobName)
	fmt.Printf("Build:      #%d\n", info.Number)
	fmt.Printf("Status:     %-12s\n", status)
	fmt.Printf("Started:    %s%s\n", startTime, timeAgo)
	fmt.Printf("Duration:   %s\n", durationStr)
	if len(info.Causes) > 0 {
		fmt.Printf("Triggered:  %s\n", strings.Join(info.Causes, "; "))
	}
	if info.GitCommit != "" {
		branchInfo := ""
		if info.GitBranch != "" {
			branchInfo = fmt.Sprintf(" (%s)", info.GitBranch)
		}
		fmt.Printf("Git Commit: %s%s\n", info.GitCommit, branchInfo)
	}
	if info.URL != "" {
		fmt.Printf("URL:        %s\n", info.URL)
	}

	if len(info.Parameters) > 0 {
		fmt.Println("\nParameters:")
		keys := make([]string, 0, len(info.Parameters))
		for k := range info.Parameters {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Printf("  %-14s %s\n", k+":", info.Parameters[k])
		}
	}

	return nil
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)
	m := d / time.Minute
	s := (d % time.Minute) / time.Second
	if m > 0 {
		return fmt.Sprintf("%dm %02ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func timeSinceCompact(t time.Time) string {
	diff := time.Since(t).Round(time.Minute)
	if diff < time.Minute {
		return "just now"
	}
	if diff < time.Hour {
		return fmt.Sprintf("%dm", int(diff.Minutes()))
	}
	if diff < 24*time.Hour {
		return fmt.Sprintf("%dh", int(diff.Hours()))
	}
	return fmt.Sprintf("%dd", int(diff.Hours()/24))
}

func init() {
	buildCmd.Flags().StringArrayVarP(&buildParamFlag, "param", "p", nil, "build parameter in key=value format (repeatable)")
	buildCmd.Flags().IntVarP(&buildLimitFlag, "limit", "n", 10, "maximum number of builds to list")
	buildCmd.Flags().BoolVar(&buildJSONFlag, "json", false, "output raw JSON format")

	buildLsCmd.Flags().IntVarP(&buildLimitFlag, "limit", "n", 10, "maximum number of builds to list")
	buildLsCmd.Flags().BoolVar(&buildJSONFlag, "json", false, "output raw JSON format")

	buildInfoCmd.Flags().BoolVar(&buildJSONFlag, "json", false, "output raw JSON format")

	buildCmd.AddCommand(buildLsCmd)
	buildCmd.AddCommand(buildInfoCmd)

	rootCmd.AddCommand(buildCmd)
}
