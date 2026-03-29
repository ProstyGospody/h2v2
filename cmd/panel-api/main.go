package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"h2v2/internal/app"
	"h2v2/internal/config"
)

func main() {
	os.Exit(run())
}

func run() int {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		return 1
	}

	logger := newLogger(cfg.Env)
	command := "serve"
	if len(os.Args) > 1 {
		command = strings.TrimSpace(os.Args[1])
	}

	ctx := context.Background()

	switch command {
	case "serve":
		if err := runServe(ctx, cfg, logger); err != nil {
			logger.Error("server exited with error", "error", err)
			return 1
		}
		return 0
	case "bootstrap-admin":
		fs := flag.NewFlagSet("bootstrap-admin", flag.ContinueOnError)
		email := fs.String("email", "", "initial admin email")
		password := fs.String("password", "", "initial admin password")
		if err := fs.Parse(os.Args[2:]); err != nil {
			fmt.Fprintf(os.Stderr, "failed to parse flags: %v\n", err)
			return 1
		}
		if strings.TrimSpace(*email) == "" || strings.TrimSpace(*password) == "" {
			fmt.Fprintln(os.Stderr, "email and password are required")
			return 1
		}
		if err := app.BootstrapAdmin(ctx, cfg, *email, *password); err != nil {
			logger.Error("bootstrap-admin failed", "error", err)
			return 1
		}
		logger.Info("admin account prepared", "email", *email)
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", command)
		fmt.Fprintln(os.Stderr, "available commands: serve, bootstrap-admin")
		return 1
	}
}

func runServe(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	repo, err := app.OpenRepository(cfg)
	if err != nil {
		return err
	}
	server := app.NewServer(cfg, logger, repo)

	sigCtx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.Run(sigCtx)
	}()

	select {
	case err := <-errCh:
		if err != nil {
			return err
		}
	case <-sigCtx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown failed: %w", err)
		}
	}
	return nil
}

func newLogger(env string) *slog.Logger {
	level := slog.LevelInfo
	if strings.EqualFold(env, "development") {
		level = slog.LevelDebug
	}
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	return slog.New(handler)
}
