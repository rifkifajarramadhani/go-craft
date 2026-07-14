package scaffold

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"golang.org/x/mod/modfile"
	"golang.org/x/mod/module"
)

const (
	TemplateVersion  = "0.1.0"
	GeneratorVersion = "0.1.0"
	baseModule       = "github.com/rifkifajarramadhani/golang-clean-architecture"
)

var projectPattern = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

type Options struct {
	Name      string
	Module    string
	Directory string
	Framework string
	Database  string
	Queue     string
	Scheduler bool
	SkipTidy  bool
}

type Metadata struct {
	SchemaVersion    int    `json:"schema_version"`
	GeneratorVersion string `json:"generator_version"`
	TemplateVersion  string `json:"template_version"`
	Module           string `json:"module"`
	Framework        string `json:"framework"`
	Database         string `json:"database"`
	Queue            string `json:"queue"`
	Scheduler        bool   `json:"scheduler"`
}

type Result struct {
	Destination string
	TidyErr     error
}

func Generate(ctx context.Context, options Options) (Result, error) {
	if err := validate(options); err != nil {
		return Result{}, err
	}

	parent, err := filepath.Abs(options.Directory)
	if err != nil {
		return Result{}, fmt.Errorf("resolve destination parent: %w", err)
	}

	if err := os.MkdirAll(parent, 0o755); err != nil {
		return Result{}, fmt.Errorf("create destination parent: %w", err)
	}

	destination := filepath.Join(parent, options.Name)
	if _, err := os.Lstat(destination); err == nil {
		return Result{}, fmt.Errorf("destination already exists: %s", destination)
	} else if !errors.Is(err, os.ErrNotExist) {
		return Result{}, fmt.Errorf("inspect destination: %w", err)
	}

	staging, err := os.MkdirTemp(parent, ".go-craft-")
	if err != nil {
		return Result{}, fmt.Errorf("create staging directory: %w", err)
	}

	committed := false
	defer func() {
		if !committed {
			_ = os.RemoveAll(staging)
		}
	}()

	if err := materialize(staging, options); err != nil {
		return Result{}, err
	}

	if err := formatGo(ctx, staging); err != nil {
		return Result{}, err
	}

	if err := os.Rename(staging, destination); err != nil {
		return Result{}, fmt.Errorf("commit generated project: %w", err)
	}
	committed = true

	result := Result{Destination: destination}
	if !options.SkipTidy {
		command := exec.CommandContext(ctx, "go", "mod", "tidy")
		command.Dir = destination
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr
		result.TidyErr = command.Run()
	}

	return result, nil
}

func validate(options Options) error {
	if !projectPattern.MatchString(options.Name) {
		return errors.New("project name must start with a lowercase letter and contain only lowercase letters, digits, and hyphens")
	}

	if err := module.CheckPath(options.Module); err != nil {
		return fmt.Errorf("invalid module path: %w", err)
	}

	if !oneOf(options.Framework, "nethttp", "fiber", "gin", "echo") {
		return fmt.Errorf("unsupported framework %q", options.Framework)
	}

	if !oneOf(options.Database, "mysql", "postgres", "sqlite") {
		return fmt.Errorf("unsupported database %q", options.Database)
	}

	if !oneOf(options.Queue, "none", "database", "redis") {
		return fmt.Errorf("unsupported queue %q", options.Queue)
	}

	if options.Scheduler && options.Queue == "none" {
		return errors.New("scheduler requires a database or redis queue")
	}

	return nil
}

func materialize(destination string, options Options) error {
	if err := copyTree("_assets/template", destination, options); err != nil {
		return fmt.Errorf("copy base template: %w", err)
	}

	overlays := []string{
		"_assets/overlays/framework/" + options.Framework,
		"_assets/overlays/database/" + options.Database,
		"_assets/overlays/queue/" + options.Queue,
	}
	if options.Framework != "fiber" {
		overlays = append([]string{"_assets/overlays/framework/common"}, overlays...)
	}
	if options.Scheduler {
		overlays = append(overlays, "_assets/overlays/scheduler/enabled")
	} else {
		overlays = append(overlays, "_assets/overlays/scheduler/disabled")
	}

	for _, overlay := range overlays {
		if err := applyOverlay(overlay, destination, options); err != nil {
			return err
		}
	}

	if err := configureCommands(destination, options); err != nil {
		return err
	}
	if err := configureProjectFiles(destination, options); err != nil {
		return err
	}

	if err := configureGoMod(destination, options); err != nil {
		return err
	}

	metadata := Metadata{
		SchemaVersion: 1, GeneratorVersion: GeneratorVersion, TemplateVersion: TemplateVersion,
		Module: options.Module, Framework: options.Framework, Database: options.Database,
		Queue: options.Queue, Scheduler: options.Scheduler,
	}
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("encode generation metadata: %w", err)
	}

	if err := os.WriteFile(filepath.Join(destination, ".go-craft.json"), append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("write generation metadata: %w", err)
	}

	return nil
}

func configureGoMod(destination string, options Options) error {
	path := filepath.Join(destination, "go.mod")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read generated go.mod: %w", err)
	}

	file, err := modfile.Parse(path, data, nil)
	if err != nil {
		return fmt.Errorf("parse generated go.mod: %w", err)
	}

	for _, dependency := range []string{
		"github.com/gofiber/fiber/v3", "github.com/gin-gonic/gin", "github.com/labstack/echo/v4",
	} {
		if err := file.DropRequire(dependency); err != nil {
			return fmt.Errorf("drop framework dependency %s: %w", dependency, err)
		}
	}

	switch options.Framework {
	case "fiber":
		err = file.AddRequire("github.com/gofiber/fiber/v3", "v3.4.0")
	case "gin":
		err = file.AddRequire("github.com/gin-gonic/gin", "v1.12.0")
	case "echo":
		err = file.AddRequire("github.com/labstack/echo/v4", "v4.15.4")
	}
	if err != nil {
		return fmt.Errorf("add selected framework dependency: %w", err)
	}

	for _, dependency := range []string{
		"github.com/go-sql-driver/mysql", "gorm.io/driver/mysql", "gorm.io/driver/postgres",
		"github.com/ncruces/go-sqlite3", "github.com/ncruces/go-sqlite3/gormlite",
	} {
		if err := file.DropRequire(dependency); err != nil {
			return fmt.Errorf("drop database dependency %s: %w", dependency, err)
		}
	}

	switch options.Database {
	case "mysql":
		if err = file.AddRequire("github.com/go-sql-driver/mysql", "v1.10.0"); err == nil {
			err = file.AddRequire("gorm.io/driver/mysql", "v1.6.0")
		}
	case "postgres":
		err = file.AddRequire("gorm.io/driver/postgres", "v1.6.0")
	case "sqlite":
		err = file.AddRequire("github.com/ncruces/go-sqlite3/gormlite", "v0.34.0")
	}
	if err != nil {
		return fmt.Errorf("add selected database dependency: %w", err)
	}

	if err := file.DropRequire("github.com/hibiken/asynq"); err != nil {
		return fmt.Errorf("drop queue dependency: %w", err)
	}
	if options.Queue == "redis" {
		if err := file.AddRequire("github.com/hibiken/asynq", "v0.26.0"); err != nil {
			return fmt.Errorf("add redis queue dependency: %w", err)
		}
	}

	formatted, err := file.Format()
	if err != nil {
		return fmt.Errorf("format generated go.mod: %w", err)
	}
	if err := os.WriteFile(path, formatted, 0o644); err != nil {
		return fmt.Errorf("write generated go.mod: %w", err)
	}
	if err := os.Remove(filepath.Join(destination, "go.sum")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove template checksums before tidy: %w", err)
	}

	return nil
}

func configureCommands(destination string, options Options) error {
	commands := []string{"serveCommand()"}
	if options.Queue != "none" {
		commands = append(commands, "workerCommand()", "allCommand()", "queueCommand()")
	}
	if options.Scheduler {
		commands = append(commands, "schedulerCommand()", "scheduleCommand()")
	}

	root := `package main

import (
	"os"

	"github.com/spf13/cobra"
)

func main() {
	if err := newRootCommand().Execute(); err != nil {
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:          "app",
		Short:        "Run and operate the application in any of its roles",
		SilenceUsage: true,
	}
	root.AddCommand(
		` + strings.Join(commands, ",\n\t\t") + `,
	)

	return root
}
`
	if err := os.WriteFile(filepath.Join(destination, "cmd/app/main.go"), []byte(root), 0o644); err != nil {
		return fmt.Errorf("write generated command root: %w", err)
	}

	if options.Queue == "none" {
		for _, path := range []string{"cmd/app/all.go", "cmd/app/queue.go", "cmd/app/worker.go"} {
			if err := os.RemoveAll(filepath.Join(destination, path)); err != nil {
				return fmt.Errorf("remove unselected queue command %s: %w", path, err)
			}
		}
	} else if !options.Scheduler {
		if err := writeServerWorkerAllCommand(destination, options.Module); err != nil {
			return err
		}
	}

	if !options.Scheduler {
		for _, path := range []string{
			"cmd/app/schedule.go", "cmd/app/scheduler.go", "internal/adapter/cron", "internal/scheduler",
			"internal/bootstrap/scheduler.go",
		} {
			if err := os.RemoveAll(filepath.Join(destination, path)); err != nil {
				return fmt.Errorf("remove unselected scheduler path %s: %w", path, err)
			}
		}
	}

	return nil
}

func writeServerWorkerAllCommand(destination, modulePath string) error {
	const source = `package main

import (
	"context"
	"fmt"
	"os/signal"
	"sync"
	"syscall"

	"__GO_CRAFT_MODULE__/internal/adapter/database"
	"__GO_CRAFT_MODULE__/internal/adapter/logging"
	"__GO_CRAFT_MODULE__/internal/bootstrap"
	"__GO_CRAFT_MODULE__/internal/config"
	"github.com/spf13/cobra"
)

func allCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "all",
		Short: "Run the server and worker in a single process",
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

			dispatcher, err := bootstrap.Dispatcher(cfg, db)
			if err != nil {
				return fmt.Errorf("build queue dispatcher: %w", err)
			}

			if closer, ok := dispatcher.(interface{ Close() error }); ok {
				defer func() { _ = closer.Close() }()
			}

			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			var wg sync.WaitGroup

			errs := make(chan error, 2)
			run := func(name string, fn func() error) {
				defer wg.Done()
				defer cancel()

				if runErr := fn(); runErr != nil {
					errs <- fmt.Errorf("%s: %w", name, runErr)
				}
			}

			wg.Add(2)

			go run("server", func() error { return runServer(ctx, cfg, logger, db, dispatcher) })
			go run("worker", func() error { return runWorker(ctx, cfg, logger, db) })

			wg.Wait()
			close(errs)

			for componentErr := range errs {
				return componentErr
			}

			return nil
		},
	}
}
`
	path := filepath.Join(destination, "cmd/app/all.go")
	data := strings.ReplaceAll(source, "__GO_CRAFT_MODULE__", modulePath)
	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		return fmt.Errorf("write combined server/worker command: %w", err)
	}

	return nil
}

func applyOverlay(root, destination string, options Options) error {
	if _, err := fs.Stat(assets, root); errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("inspect overlay %s: %w", root, err)
	}

	removeFile := root + "/.remove"
	if data, err := fs.ReadFile(assets, removeFile); err == nil {
		lines := strings.Split(string(data), "\n")
		sort.Strings(lines)
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}

			if err := os.RemoveAll(filepath.Join(destination, filepath.FromSlash(line))); err != nil {
				return fmt.Errorf("remove %s for overlay %s: %w", line, root, err)
			}
		}
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("read overlay removals: %w", err)
	}

	return copyTree(root, destination, options)
}

func copyTree(root, destination string, options Options) error {
	return fs.WalkDir(assets, root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		relative, err := filepath.Rel(root, path)
		if err != nil || relative == "." {
			return err
		}

		if relative == ".remove" || relative == ".keep" || strings.HasPrefix(relative, ".craft") || ignored(relative) {
			if entry.IsDir() {
				return fs.SkipDir
			}

			return nil
		}

		targetRelative := relative
		if filepath.Base(relative) == "go.mod.tmpl" {
			targetRelative = strings.TrimSuffix(relative, ".tmpl")
		}
		target := filepath.Join(destination, filepath.FromSlash(targetRelative))
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}

		data, err := fs.ReadFile(assets, path)
		if err != nil {
			return err
		}

		data = render(data, options)
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}

		return os.WriteFile(target, data, 0o644)
	})
}

func ignored(relative string) bool {
	first := strings.Split(filepath.ToSlash(relative), "/")[0]
	return oneOf(first, ".git", ".env", "dist", "logs", "app")
}

func render(data []byte, options Options) []byte {
	if bytes.IndexByte(data, 0) >= 0 || !utf8.Valid(data) {
		return data
	}

	value := string(data)
	value = strings.ReplaceAll(value, baseModule, options.Module)
	value = strings.ReplaceAll(value, "__GO_CRAFT_MODULE__", options.Module)
	value = strings.ReplaceAll(value, "__GO_CRAFT_PROJECT__", options.Name)

	return []byte(value)
}

func formatGo(ctx context.Context, directory string) error {
	command := exec.CommandContext(ctx, "gofmt", "-w", ".")
	command.Dir = directory
	if output, err := command.CombinedOutput(); err != nil {
		return fmt.Errorf("format generated Go files: %w: %s", err, strings.TrimSpace(string(output)))
	}

	return nil
}

func oneOf(value string, options ...string) bool {
	for _, option := range options {
		if value == option {
			return true
		}
	}

	return false
}
