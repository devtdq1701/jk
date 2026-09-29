package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/devtdq1701/jk/pkg/config"
	"github.com/devtdq1701/jk/pkg/jenkins"
)

var (
	cfgPath       string
	targetContext string
	timeout       time.Duration
	rateLimitRPS  float64

	rootCmd = &cobra.Command{
		Use:   "jk",
		Short: "Lightweight, scriptable Jenkins CLI",
		Long:  "jk is a fast CLI for managing Jenkins contexts, streaming logs, patching configs, and inspecting builds.",
	}
)

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgPath, "config", "", "path to config file (default ~/.config/jk/config.json)")
	rootCmd.PersistentFlags().StringVar(&targetContext, "context", "", "override active context")
	rootCmd.PersistentFlags().DurationVar(&timeout, "timeout", 15*time.Second, "http request timeout")
	rootCmd.PersistentFlags().Float64Var(&rateLimitRPS, "rate-limit", 5.0, "maximum requests per second")
}

func getClient() (*jenkins.Client, error) {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}

	_, ctx, err := cfg.ResolveContext(targetContext)
	if err != nil {
		return nil, err
	}

	return jenkins.NewClient(ctx.URL, ctx.Username, ctx.Token, timeout, rateLimitRPS, 10)
}
