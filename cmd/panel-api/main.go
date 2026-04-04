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
	"h2v2/internal/repository"
)

func main() {
	os.Exit(run())
}

func run() int {
	command := "serve"
	if len(os.Args) > 1 {
		command = strings.TrimSpace(os.Args[1])
	}
	if command == "" {
		command = "serve"
	}

	ctx := context.Background()

	switch command {
	case "serve":
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "config error: %v\n", err)
			return 1
		}
		logger := newLogger(cfg.Env)
		if err := runServe(ctx, cfg, logger); err != nil {
			logger.Error("server exited with error", "error", err)
			return 1
		}
		return 0

	case "bootstrap-admin":
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "config error: %v\n", err)
			return 1
		}
		logger := newLogger(cfg.Env)
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

	case "bootstrap-inbounds":
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "config error: %v\n", err)
			return 1
		}
		logger := newLogger(cfg.Env)
		if err := app.BootstrapInbounds(ctx, cfg); err != nil {
			logger.Error("bootstrap-inbounds failed", "error", err)
			return 1
		}
		logger.Info("default inbounds prepared")
		return 0

	case "refresh-inbounds":
		cfg, err := config.Load()
		if err != nil {
			fmt.Fprintf(os.Stderr, "config error: %v\n", err)
			return 1
		}
		logger := newLogger(cfg.Env)
		if err := app.RefreshInbounds(ctx, cfg); err != nil {
			logger.Error("refresh-inbounds failed", "error", err)
			return 1
		}
		logger.Info("inbounds refreshed and sing-box reloaded")
		return 0

	case "sqlite-backup":
		return runSQLiteBackup(ctx, os.Args[2:])
	case "export":
		return runSQLiteExport(ctx, os.Args[2:])
	case "sqlite-restore":
		return runSQLiteRestore(os.Args[2:])

	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", command)
		fmt.Fprintln(os.Stderr, "available commands: serve, bootstrap-admin, bootstrap-inbounds, refresh-inbounds, sqlite-backup, export, sqlite-restore")
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

func runSQLiteBackup(ctx context.Context, args []string) int {
	defaultRoot := firstNonEmpty(os.Getenv("PANEL_STORAGE_ROOT"), "/var/lib/h2v2")
	defaultDB := firstNonEmpty(
		os.Getenv("PANEL_SQLITE_PATH"),
		filepathJoin(defaultRoot, "data", "h2v2.db"),
		"/var/lib/h2v2/data/h2v2.db",
	)
	defaultOut := filepathJoin(
		defaultRoot,
		"backups",
		"panel-"+time.Now().UTC().Format("20060102-1504")+".db",
	)

	fs := flag.NewFlagSet("sqlite-backup", flag.ContinueOnError)
	db := fs.String("db", defaultDB, "sqlite db path")
	out := fs.String("out", defaultOut, "backup file path")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse flags: %v\n", err)
		return 1
	}

	repo, err := repository.NewSQLiteRepository(*db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open sqlite repository: %v\n", err)
		return 1
	}
	defer repo.Close()
	if err := repo.BackupTo(ctx, *out); err != nil {
		fmt.Fprintf(os.Stderr, "sqlite backup failed: %v\n", err)
		return 1
	}
	fmt.Printf("backup written: %s\n", *out)
	return 0
}

func runSQLiteExport(ctx context.Context, args []string) int {
	defaultRoot := firstNonEmpty(os.Getenv("PANEL_STORAGE_ROOT"), "/var/lib/h2v2")
	defaultDB := firstNonEmpty(
		os.Getenv("PANEL_SQLITE_PATH"),
		filepathJoin(defaultRoot, "data", "h2v2.db"),
		"/var/lib/h2v2/data/h2v2.db",
	)
	defaultOut := filepathJoin(
		defaultRoot,
		"backups",
		"export-"+time.Now().UTC().Format("20060102-1504")+".json",
	)

	fs := flag.NewFlagSet("export", flag.ContinueOnError)
	db := fs.String("db", defaultDB, "sqlite db path")
	out := fs.String("out", defaultOut, "export file path")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse flags: %v\n", err)
		return 1
	}

	repo, err := repository.NewSQLiteRepository(*db)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open sqlite repository: %v\n", err)
		return 1
	}
	defer repo.Close()

	counts, err := repo.ExportToJSON(ctx, *out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "export failed: %v\n", err)
		return 1
	}
	fmt.Printf("export written: %s counts=%+v\n", *out, counts)
	return 0
}

func runSQLiteRestore(args []string) int {
	defaultRoot := firstNonEmpty(os.Getenv("PANEL_STORAGE_ROOT"), "/var/lib/h2v2")
	defaultDB := firstNonEmpty(
		os.Getenv("PANEL_SQLITE_PATH"),
		filepathJoin(defaultRoot, "data", "h2v2.db"),
		"/var/lib/h2v2/data/h2v2.db",
	)

	fs := flag.NewFlagSet("sqlite-restore", flag.ContinueOnError)
	db := fs.String("db", defaultDB, "sqlite db path")
	from := fs.String("from", "", "backup db path")
	if err := fs.Parse(args); err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse flags: %v\n", err)
		return 1
	}

	rollbackPath, err := repository.SQLiteRestore(*db, *from)
	if err != nil {
		fmt.Fprintf(os.Stderr, "restore failed: %v\n", err)
		return 1
	}
	if strings.TrimSpace(rollbackPath) != "" {
		fmt.Printf("restore completed: db=%s rollback_backup=%s\n", *db, rollbackPath)
		return 0
	}
	fmt.Printf("restore completed: db=%s\n", *db)
	return 0
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func filepathJoin(parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			filtered = append(filtered, trimmed)
		}
	}
	if len(filtered) == 0 {
		return ""
	}
	joined := filtered[0]
	for _, part := range filtered[1:] {
		if strings.HasSuffix(joined, "/") {
			joined = strings.TrimRight(joined, "/")
		}
		joined += "/" + strings.TrimLeft(part, "/")
	}
	return joined
}

func newLogger(env string) *slog.Logger {
	level := slog.LevelInfo
	if strings.EqualFold(env, "development") {
		level = slog.LevelDebug
	}
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	return slog.New(handler)
}
