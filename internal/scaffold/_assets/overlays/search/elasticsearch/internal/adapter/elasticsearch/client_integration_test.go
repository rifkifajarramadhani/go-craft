package elasticsearchadapter

import (
	"context"
	"os"
	"testing"
	"time"

	"__GO_CRAFT_MODULE__/internal/config"
)

func TestOpenIntegration(t *testing.T) {
	endpoint := os.Getenv("ELASTICSEARCH_TEST_URL")
	if endpoint == "" {
		t.Skip("ELASTICSEARCH_TEST_URL is not configured")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if _, err := Open(ctx, config.ElasticsearchConfig{URL: endpoint}); err != nil {
		t.Fatal(err)
	}
}
