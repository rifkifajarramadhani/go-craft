package main

import (
	"context"

	"__GO_CRAFT_MODULE__/internal/adapter/httpapi"
	"__GO_CRAFT_MODULE__/internal/adapter/httpserver"
	"__GO_CRAFT_MODULE__/internal/adapter/logging"
	"__GO_CRAFT_MODULE__/internal/bootstrap"
	"__GO_CRAFT_MODULE__/internal/config"
	"__GO_CRAFT_MODULE__/internal/queue"
	"gorm.io/gorm"
)

func runServer(ctx context.Context, cfg *config.Config, logger *logging.Logger, db *gorm.DB, dispatcher queue.Dispatcher) error {
	services := bootstrap.WireHTTPServices(cfg, db, logger.Logger, dispatcher)
	ready := func(ctx context.Context) bool {
		sqlDB, err := db.DB()
		return err == nil && sqlDB.PingContext(ctx) == nil
	}
	api := httpapi.New(services.Users, services.Auth, services.Tokens, logger.Logger, ready)
	server := httpserver.New(cfg.App.Port, api.Handler(), logger.Logger)

	logger.Info("server running", "port", cfg.App.Port)

	return server.Run(ctx)
}
