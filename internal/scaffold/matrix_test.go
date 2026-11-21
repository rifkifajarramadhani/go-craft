package scaffold

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateDefaultsSearchToNone(t *testing.T) {
	result, err := Generate(context.Background(), Options{
		Name: "default-search", Module: "example.com/default-search", Directory: t.TempDir(),
		Framework: "nethttp", Database: "sqlite", Queue: "none", SkipTidy: true,
	})
	if err != nil {
		t.Fatal(err)
	}

	metadata := readGenerated(t, result.Destination, ".go-craft.json")
	if !strings.Contains(metadata, `"search": "none"`) {
		t.Fatalf("metadata does not record default search selection: %s", metadata)
	}
	if _, err := os.Stat(filepath.Join(result.Destination, "internal/adapter/elasticsearch")); !os.IsNotExist(err) {
		t.Fatalf("default project contains Elasticsearch adapter: %v", err)
	}
}

func TestValidateRejectsUnknownSearchAdapter(t *testing.T) {
	err := validate(Options{
		Name: "invalid-search", Module: "example.com/invalid-search",
		Framework: "nethttp", Database: "sqlite", Queue: "none", Search: "opensearch",
	})
	if err == nil || !strings.Contains(err.Error(), "unsupported search adapter") {
		t.Fatalf("error = %v, want unsupported search adapter", err)
	}
}

func TestGenerateAllValidVariants(t *testing.T) {
	frameworks := []string{"nethttp", "fiber", "gin", "echo"}
	databases := []string{"mysql", "postgres", "sqlite"}
	queues := []string{"none", "database", "redis"}
	searches := []string{"none", "elasticsearch"}
	variant := 0

	for _, framework := range frameworks {
		for _, database := range databases {
			for _, queue := range queues {
				for _, search := range searches {
					schedulers := []bool{false, true}
					if queue == "none" {
						schedulers = []bool{false}
					}

					for _, scheduler := range schedulers {
						variant++
						name := fmt.Sprintf("variant-%03d", variant)
						t.Run(name, func(t *testing.T) {
							parent := t.TempDir()
							options := Options{
								Name: name, Module: "example.com/" + name, Directory: parent,
								Framework: framework, Database: database, Queue: queue, Search: search,
								Scheduler: scheduler, SkipTidy: true,
							}
							result, err := Generate(context.Background(), options)
							if err != nil {
								t.Fatalf("generate: %v", err)
							}

							assertSelection(t, result.Destination, options)
						})
					}
				}
			}
		}
	}

	if variant != 120 {
		t.Fatalf("generated %d variants, want 120", variant)
	}
}

func assertSelection(t *testing.T, destination string, options Options) {
	t.Helper()

	goMod := readGenerated(t, destination, "go.mod")
	frameworkModules := map[string]string{
		"fiber": "github.com/gofiber/fiber/v3",
		"gin":   "github.com/gin-gonic/gin",
		"echo":  "github.com/labstack/echo/v5",
	}
	for framework, modulePath := range frameworkModules {
		want := framework == options.Framework
		if strings.Contains(goMod, modulePath) != want {
			t.Errorf("framework dependency %s presence does not match %t", modulePath, want)
		}
	}

	databasePaths := map[string]string{
		"mysql":    "internal/adapter/mysql",
		"postgres": "internal/adapter/postgres",
		"sqlite":   "internal/adapter/sqlite",
	}
	for database, path := range databasePaths {
		_, err := os.Stat(filepath.Join(destination, path))
		present := err == nil
		if present != (database == options.Database) {
			t.Errorf("database path %s presence = %t", path, present)
		}
	}

	_, queueErr := os.Stat(filepath.Join(destination, "internal/adapter/queue"))
	if (queueErr == nil) != (options.Queue != "none") {
		t.Errorf("queue adapter presence does not match %q", options.Queue)
	}
	if options.Queue == "redis" && !strings.Contains(goMod, "github.com/hibiken/asynq") {
		t.Error("redis variant does not include Asynq")
	}
	if options.Queue != "redis" && strings.Contains(goMod, "github.com/hibiken/asynq") {
		t.Error("non-redis variant includes Asynq")
	}

	elasticsearchPath := filepath.Join(destination, "internal/adapter/elasticsearch")
	_, elasticsearchErr := os.Stat(elasticsearchPath)
	elasticsearchSelected := options.Search == "elasticsearch"
	if (elasticsearchErr == nil) != elasticsearchSelected {
		t.Errorf("Elasticsearch adapter presence does not match %q", options.Search)
	}
	if strings.Contains(goMod, "github.com/elastic/go-elasticsearch/v8") != elasticsearchSelected {
		t.Errorf("Elasticsearch dependency presence does not match %q", options.Search)
	}
	queueMigration := filepath.Join(destination, "internal/adapter", options.Database, "migrations", "000003_create_queue_tables.up.sql")
	_, migrationErr := os.Stat(queueMigration)
	if (migrationErr == nil) != (options.Queue == "database") {
		t.Errorf("database queue migration presence does not match %q", options.Queue)
	}

	_, schedulerErr := os.Stat(filepath.Join(destination, "internal/scheduler"))
	if (schedulerErr == nil) != options.Scheduler {
		t.Errorf("scheduler presence does not match %t", options.Scheduler)
	}

	metadata := readGenerated(t, destination, ".go-craft.json")
	if strings.Contains(metadata, `"search": "`+options.Search+`"`) == false {
		t.Errorf("metadata does not contain search selection %q", options.Search)
	}
	for _, selection := range []string{options.Framework, options.Database, options.Queue} {
		if !strings.Contains(metadata, `"`+selection+`"`) {
			t.Errorf("metadata does not contain selection %q", selection)
		}
	}
	readme := readGenerated(t, destination, "README.md")
	for _, selection := range []string{options.Framework, options.Database, options.Queue, options.Search} {
		if !strings.Contains(readme, "`"+selection+"`") {
			t.Errorf("README does not contain selection %q", selection)
		}
	}

	config := readGenerated(t, destination, "configs/config.yaml")
	compose := readGenerated(t, destination, "docker-compose.yml")
	environment := readGenerated(t, destination, ".env.example")
	for name, content := range map[string]string{"config": config, "compose": compose, "environment": environment} {
		if strings.Contains(content, "ELASTICSEARCH") || strings.Contains(content, "elasticsearch:") {
			if !elasticsearchSelected {
				t.Errorf("%s contains Elasticsearch configuration for unselected adapter", name)
			}
		} else if elasticsearchSelected {
			t.Errorf("%s does not contain selected Elasticsearch configuration", name)
		}
	}
}

func readGenerated(t *testing.T, destination, relative string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(destination, relative))
	if err != nil {
		t.Fatalf("read %s: %v", relative, err)
	}

	return string(data)
}
