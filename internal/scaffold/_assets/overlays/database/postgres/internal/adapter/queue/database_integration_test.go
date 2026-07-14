package queueinfra

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"__GO_CRAFT_MODULE__/internal/config"
	"__GO_CRAFT_MODULE__/internal/queue"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type postgresIntegrationJob struct {
	Message string `json:"message"`
}

func (postgresIntegrationJob) Type() string     { return "test:postgres-database-queue" }
func (job postgresIntegrationJob) Payload() any { return job }

func TestPostgresDatabaseQueueDispatchReserveAndComplete(t *testing.T) {
	dsn := os.Getenv("QUEUE_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("QUEUE_TEST_POSTGRES_DSN is not set")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatal(err)
	}

	for _, table := range []any{&databaseJob{}, &databaseLock{}, &databaseStat{}} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("required queue table for %T is missing; apply database migrations first", table)
		}
	}

	ctx := context.Background()
	taskID := "postgres-queue-" + time.Now().Format("20060102150405.000000000")
	dispatcher := NewDatabaseDispatcher(db)

	_, err = dispatcher.Dispatch(ctx, postgresIntegrationJob{Message: "hello"}, queue.DispatchOptions{
		Queue: "default", TaskID: taskID,
	})
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		db.Where("job_id = ?", taskID).Delete(&databaseLock{})
		db.Where("id = ?", taskID).Delete(&databaseJob{})
	})

	registry := queue.NewHandlerRegistry()
	if err := registry.Register(postgresIntegrationJob{}.Type(), func(context.Context, json.RawMessage) error { return nil }); err != nil {
		t.Fatal(err)
	}

	worker := NewDatabaseWorker(db, config.QueueConfig{
		Concurrency: 1,
		Queues:      map[string]int{"default": 1},
		Database: config.DatabaseQueueConfig{
			PollIntervalMilliseconds: 10,
			ReservationSeconds:       5,
		},
	}, registry, nil)

	job, err := worker.reserve(ctx, "default")
	if err != nil {
		t.Fatal(err)
	}

	if job == nil || job.ID != taskID {
		t.Fatalf("reserved job = %#v, want %s", job, taskID)
	}

	if err := worker.complete(job); err != nil {
		t.Fatal(err)
	}
}
