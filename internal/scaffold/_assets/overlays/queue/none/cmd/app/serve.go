package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"

	"__GO_CRAFT_MODULE__/internal/adapter/database"
	"__GO_CRAFT_MODULE__/internal/adapter/logging"
	"__GO_CRAFT_MODULE__/internal/config"
	"github.com/spf13/cobra"
)

func serveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run the HTTP API server",
		RunE: func(_ *cobra.Command, _ []string) error {
			ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer stop()

			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			logger, err := logging.New(cfg.Logging)
			if err != nil {
				return fmt.Errorf("create logger: %w", err)
			}
			defer func() { _ = logger.Close() }()

			db, err := database.Open(ctx, cfg.Database, logger.Logger)
			if err != nil {
				return fmt.Errorf("connect to database: %w", err)
			}
			defer func() { _ = database.Close(db) }()

			return runServer(ctx, cfg, logger, db, nil)
		},
	}
}
