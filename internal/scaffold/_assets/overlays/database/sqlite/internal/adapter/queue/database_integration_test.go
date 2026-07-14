package queueinfra

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	sqliteadapter "__GO_CRAFT_MODULE__/internal/adapter/sqlite"
	"__GO_CRAFT_MODULE__/internal/config"
	"__GO_CRAFT_MODULE__/internal/queue"
	"gorm.io/gorm"
)

const sqliteQueueHelperEnvironment = "GO_CRAFT_SQLITE_QUEUE_HELPER"

type sqliteStressJob struct {
	Sequence int `json:"sequence"`
}

func (sqliteStressJob) Type() string     { return "test:sqlite-stress" }
func (job sqliteStressJob) Payload() any { return job }

func TestSQLiteDatabaseQueueMultiProcessClaimsExactlyOnce(t *testing.T) {
	if os.Getenv(sqliteQueueHelperEnvironment) != "" {
		t.Skip("parent-only test")
	}

	path := filepath.Join(t.TempDir(), "queue.db")
	db := openSQLiteQueueTestDatabase(t, path, true)
	dispatcher := NewDatabaseDispatcher(db)
	ctx := context.Background()

	const jobs = 40
	for sequence := range jobs {
		_, err := dispatcher.Dispatch(ctx, sqliteStressJob{Sequence: sequence}, queue.DispatchOptions{
			Queue: "default", TaskID: fmt.Sprintf("sqlite-stress-%03d", sequence),
		})
		if err != nil {
			t.Fatalf("dispatch job %d: %v", sequence, err)
		}
	}

	outputs := make(chan string, 4)
	errors := make(chan error, 4)

	var processes sync.WaitGroup
	for range 4 {
		processes.Add(1)

		go func() {
			defer processes.Done()

			command := exec.Command(os.Args[0], "-test.run=^TestSQLiteDatabaseQueueProcessHelper$", "-test.v")

			command.Env = append(os.Environ(), sqliteQueueHelperEnvironment+"="+path)

			output, err := command.CombinedOutput()
			if err != nil {
				errors <- fmt.Errorf("queue helper: %w: %s", err, output)

				return
			}

			outputs <- string(output)
		}()
	}

	processes.Wait()
	close(outputs)
	close(errors)

	for err := range errors {
		t.Fatal(err)
	}

	claimed := make(map[string]struct{}, jobs)

	for output := range outputs {
		scanner := bufio.NewScanner(strings.NewReader(output))
		for scanner.Scan() {
			const prefix = "CLAIMED:"
			if !strings.HasPrefix(scanner.Text(), prefix) {
				continue
			}

			id := strings.TrimPrefix(scanner.Text(), prefix)
			if _, duplicate := claimed[id]; duplicate {
				t.Fatalf("job %s was claimed more than once", id)
			}

			claimed[id] = struct{}{}
		}

		if err := scanner.Err(); err != nil {
			t.Fatal(err)
		}
	}

	if len(claimed) != jobs {
		t.Fatalf("claimed %d jobs, want %d", len(claimed), jobs)
	}

	var remaining int64
	if err := db.Model(&databaseJob{}).Count(&remaining).Error; err != nil {
		t.Fatal(err)
	}

	if remaining != 0 {
		t.Fatalf("remaining jobs = %d, want 0", remaining)
	}
}

func TestSQLiteDatabaseQueueProcessHelper(t *testing.T) {
	path := os.Getenv(sqliteQueueHelperEnvironment)
	if path == "" {
		t.Skip("helper process only")
	}

	db := openSQLiteQueueTestDatabase(t, path, false)
	worker := NewDatabaseWorker(db, config.QueueConfig{
		Concurrency: 1,
		Queues:      map[string]int{"default": 1},
		Database: config.DatabaseQueueConfig{
			PollIntervalMilliseconds: 10,
			ReservationSeconds:       5,
		},
	}, queue.NewHandlerRegistry(), nil)

	ctx := context.Background()
	for {
		job, err := worker.reserve(ctx, "default")
		if err != nil {
			t.Fatal(err)
		}

		if job == nil {
			return
		}

		fmt.Printf("CLAIMED:%s\n", job.ID)

		if err := worker.complete(job); err != nil {
			t.Fatal(err)
		}
	}
}

func openSQLiteQueueTestDatabase(t *testing.T, path string, migrate bool) *gorm.DB {
	t.Helper()

	db, err := sqliteadapter.Open(context.Background(), path, nil)
	if err != nil {
		t.Fatal(err)
	}

	if migrate {
		if err := db.AutoMigrate(&databaseJob{}, &databaseLock{}, &databaseStat{}); err != nil {
			t.Fatal(err)
		}

		for _, statement := range []string{
			"CREATE UNIQUE INDEX IF NOT EXISTS test_queue_jobs_id ON queue_jobs(id)",
			"CREATE UNIQUE INDEX IF NOT EXISTS test_queue_locks_key ON queue_locks(lock_key)",
			"CREATE UNIQUE INDEX IF NOT EXISTS test_queue_stats_queue ON queue_stats(queue)",
		} {
			if err := db.Exec(statement).Error; err != nil {
				t.Fatal(err)
			}
		}
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = sqlDB.Close() })

	return db
}
