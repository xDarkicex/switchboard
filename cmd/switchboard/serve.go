package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/xDarkicex/switchboard"
	"github.com/xDarkicex/switchboard/internal/board"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Run the switchboard proxy.",
	Long: `Load the config, mount the pipeline, and come on duty.

The operator listens on the address in switchboard.yaml. SIGINT ends the shift.`,
	RunE: runServe,
}

var flagServeConfig string

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().StringVar(&flagServeConfig, "config", "./switchboard.yaml", "Path to the switchboard config YAML")
}

func runServe(cmd *cobra.Command, _ []string) error {
	cfg, err := board.LoadFile(flagServeConfig)
	if err != nil {
		return err
	}
	srv, err := switchboard.NewFromFiles(flagServeConfig, cfg.Routing)
	if err != nil {
		return err
	}
	defer srv.Close()

	modelCount := 0
	for _, p := range cfg.Providers {
		modelCount += len(p.Models)
	}
	var unsetKeys []string
	for _, p := range cfg.Providers {
		if p.Auth.Env != "" && p.Auth.Type != "none" && os.Getenv(p.Auth.Env) == "" {
			unsetKeys = append(unsetKeys, p.Auth.Env)
		}
	}

	printBanner(cfg.Server.Listen, len(cfg.Providers), modelCount, unsetKeys)

	gracefulCtx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.GoOnDuty() }()

	select {
	case err := <-serveErr:
		if err != nil && err != http.ErrServerClosed {
			return err
		}
	case <-gracefulCtx.Done():
		fmt.Fprintf(os.Stderr, "\nswitchboard: going off duty. shift complete.\n\n")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.GoOffDuty(shutdownCtx)
	}
	return nil
}

const (
	kReset  = "\033[0m"
	kBold   = "\033[1m"
	kDim    = "\033[2m"
	kRed    = "\033[31m"
	kGreen  = "\033[32m"
	kYellow = "\033[33m"
	kCyan   = "\033[36m"
)

func printBanner(listen string, providers, models int, unsetKeys []string) {
	cyan := func(s string) string { return kCyan + s + kReset }
	bold := func(s string) string { return kBold + s + kReset }
	green := func(s string) string { return kGreen + s + kReset }

	fmt.Fprintf(os.Stderr, "\n%s", cyan(strings.Repeat("─", 58)))
	fmt.Fprintf(os.Stderr, "\n%s", bold(`
  ██████╗ ██╗    ██╗██╗████████╗ ██████╗██╗  ██╗██████╗  ██████╗  █████╗ ██████╗ ██████╗
  ██╔════╝██║    ██║██║╚══██╔══╝██╔════╝██║  ██║██╔══██╗██╔═══██╗██╔══██╗██╔══██╗██╔══██╗
  ╚█████╗ ██║ █╗ ██║██║   ██║   ██║     ███████║██████╔╝██║   ██║███████║██████╔╝██║  ██║
   ╚═══██╗██║███╗██║██║   ██║   ██║     ██╔══██║██╔══██╗██║   ██║██╔══██║██╔══██╗██║  ██║
  ██████╔╝╚███╔███╔╝██║   ██║   ╚██████╗██║  ██║██████╔╝╚██████╔╝██║  ██║██║  ██║██████╔╝
  ╚═════╝  ╚══╝╚══╝ ╚═╝   ╚═╝    ╚═════╝╚═╝  ╚═╝╚═════╝  ╚═════╝ ╚═╝  ╚═╝╚═╝  ╚═╝╚═════╝ `))
	fmt.Fprintf(os.Stderr, "\n%s", kReset)
	fmt.Fprintf(os.Stderr, "\n%s", cyan(strings.Repeat("─", 58)))
	fmt.Fprintf(os.Stderr, "\n\n")
	fmt.Fprintf(os.Stderr, "  %s  ·  MIT  ·  xDarkicex  ·  libravdb.com\n", bold("switchboard v0.1.0"))
	fmt.Fprintf(os.Stderr, "  %s  ·  %s%d providers%s  ·  %s%d models%s\n",
		green("listening on "+listen), kBold, providers, kReset, kBold, models, kReset)
	if len(unsetKeys) > 0 {
		fmt.Fprintf(os.Stderr, "  %s%s%s\n", kRed, strings.Join(unsetKeys, ", ")+" unset — calls will be dropped", kReset)
	}
	fmt.Fprintf(os.Stderr, "  %sclassify → tag → route → patch%s\n", kDim, kReset)
	fmt.Fprintf(os.Stderr, "\n")
}
