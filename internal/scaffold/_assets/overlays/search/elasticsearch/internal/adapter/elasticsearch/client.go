// Package elasticsearchadapter creates and checks the generated service's Elasticsearch client.
package elasticsearchadapter

import (
	"context"
	"fmt"
	"io"
	"time"

	"__GO_CRAFT_MODULE__/internal/config"
	elasticsearch "github.com/elastic/go-elasticsearch/v8"
)

// Open creates an official Elasticsearch client and verifies cluster availability.
func Open(ctx context.Context, cfg config.ElasticsearchConfig) (*elasticsearch.Client, error) {
	client, err := newClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("create Elasticsearch client: %w", err)
	}

	if err := Check(ctx, client); err != nil {
		return nil, fmt.Errorf("check Elasticsearch cluster: %w", err)
	}

	return client, nil
}

func newClient(cfg config.ElasticsearchConfig) (*elasticsearch.Client, error) {
	return elasticsearch.NewClient(elasticsearch.Config{
		Addresses:     []string{cfg.URL},
		Username:      cfg.Username,
		Password:      cfg.Password,
		RetryOnStatus: []int{429, 502, 503, 504},
		MaxRetries:    5,
		RetryBackoff: func(attempt int) time.Duration {
			return time.Duration(attempt+1) * 100 * time.Millisecond
		},
	})
}

// Check verifies that Elasticsearch accepts an Info request using the supplied context.
func Check(ctx context.Context, client *elasticsearch.Client) error {
	if client == nil {
		return fmt.Errorf("elasticsearch client is nil")
	}

	response, err := client.Info(client.Info.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("request Elasticsearch info: %w", err)
	}

	_, drainErr := io.Copy(io.Discard, response.Body)

	closeErr := response.Body.Close()

	if response.IsError() {
		return fmt.Errorf("elasticsearch info returned HTTP status %d", response.StatusCode)
	}

	if drainErr != nil {
		return fmt.Errorf("read Elasticsearch info response: %w", drainErr)
	}

	if closeErr != nil {
		return fmt.Errorf("close Elasticsearch info response: %w", closeErr)
	}

	return nil
}
