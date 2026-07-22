package main

import (
	"context"
	"time"

	elasticsearchadapter "__GO_CRAFT_MODULE__/internal/adapter/elasticsearch"
	"__GO_CRAFT_MODULE__/internal/config"
)

type searchReadiness func(context.Context) error

func openSearch(ctx context.Context, cfg *config.Config) (searchReadiness, error) {
	startupCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	client, err := elasticsearchadapter.Open(startupCtx, cfg.Elasticsearch)
	if err != nil {
		return nil, err
	}

	return func(ctx context.Context) error {
		return elasticsearchadapter.Check(ctx, client)
	}, nil
}
