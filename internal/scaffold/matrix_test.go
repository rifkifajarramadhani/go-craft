package scaffold

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateAllValidVariants(t *testing.T) {
	frameworks := []string{"nethttp", "fiber", "gin", "echo"}
	databases := []string{"mysql", "postgres", "sqlite"}
	queues := []string{"none", "database", "redis"}
	variant := 0

	for _, framework := range frameworks {
		for _, database := range databases {
			for _, queue := range queues {
				schedulers := []bool{false, true}
				if queue == "none" {
					schedulers = []bool{false}
				}

				for _, scheduler := range schedulers {
					variant++
					name := fmt.Sprintf("variant-%02d", variant)
					t.Run(name, func(t *testing.T) {
						parent := t.TempDir()
						options := Options{
							Name: name, Module: "example.com/" + name, Directory: parent,
							Framework: framework, Database: database, Queue: queue,
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

	if variant != 60 {
		t.Fatalf("generated %d variants, want 60", variant)
	}
}

func assertSelection(t *testing.T, destination string, options Options) {
	t.Helper()

	goMod := readGenerated(t, destination, "go.mod")
	frameworkModules := map[string]string{
		"fiber": "github.com/gofiber/fiber/v3",
		"gin":   "github.com/gin-gonic/gin",
		"echo":  "github.com/labstack/echo/v4",
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
	for _, selection := range []string{options.Framework, options.Database, options.Queue} {
		if !strings.Contains(metadata, `"`+selection+`"`) {
			t.Errorf("metadata does not contain selection %q", selection)
		}
	}
	readme := readGenerated(t, destination, "README.md")
	for _, selection := range []string{options.Framework, options.Database, options.Queue} {
		if !strings.Contains(readme, "`"+selection+"`") {
			t.Errorf("README does not contain selection %q", selection)
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
