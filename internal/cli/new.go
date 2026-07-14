package cli

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"

	"charm.land/huh/v2"
	"github.com/rifkifajarramadhani/go-craft/internal/configstore"
	"github.com/rifkifajarramadhani/go-craft/internal/scaffold"
	"github.com/spf13/cobra"
)

var ownerPattern = regexp.MustCompile(`^[A-Za-z0-9](?:[A-Za-z0-9-]{0,38})$`)

type newOptions struct {
	module        string
	owner         string
	directory     string
	framework     string
	database      string
	queue         string
	scheduler     bool
	noInteraction bool
	accessible    bool
	skipTidy      bool
}

func newCommand(store *configstore.Store) *cobra.Command {
	options := newOptions{
		directory: ".", framework: "fiber", database: "mysql", queue: "database",
	}

	command := &cobra.Command{
		Use:   "new <name>",
		Short: "Generate a new clean-architecture Go project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runNew(cmd, store, args[0], &options)
		},
	}
	flags := command.Flags()
	flags.StringVar(&options.module, "module", "", "Complete Go module path override")
	flags.StringVar(&options.owner, "owner", "", "GitHub owner override")
	flags.StringVarP(&options.directory, "directory", "d", ".", "Parent directory for the generated project")
	flags.StringVar(&options.framework, "framework", "fiber", "HTTP framework: nethttp, fiber, gin, or echo")
	flags.StringVar(&options.database, "database", "mysql", "Database: mysql, postgres, or sqlite")
	flags.StringVar(&options.queue, "queue", "database", "Queue: none, database, or redis")
	flags.BoolVar(&options.scheduler, "scheduler", false, "Include the scheduler process")
	flags.BoolVar(&options.noInteraction, "no-interaction", false, "Disable interactive prompts")
	flags.BoolVar(&options.accessible, "accessible", false, "Use accessible line-oriented prompts")
	flags.BoolVar(&options.skipTidy, "skip-tidy", false, "Do not run go mod tidy after generation")

	return command
}

func runNew(cmd *cobra.Command, store *configstore.Store, name string, options *newOptions) error {
	cfg, err := store.Load()
	if err != nil {
		return err
	}

	owner := strings.TrimSpace(options.owner)
	if owner == "" {
		owner = cfg.GitHubOwner
	}

	interactive := !options.noInteraction && stdinIsTerminal()
	if interactive {
		if err := runWizard(cmd, options, &owner); err != nil {
			return err
		}
	}

	modulePath := strings.TrimSpace(options.module)
	if modulePath == "" {
		if err := validateOwner(owner); err != nil {
			if options.noInteraction {
				return errors.New("a GitHub owner is required; pass --owner, --module, or run go-craft config set github-owner <owner>")
			}

			return err
		}

		modulePath = "github.com/" + owner + "/" + name
	}

	if interactive && cfg.GitHubOwner == "" && options.owner == "" && owner != "" {
		if err := store.Save(configstore.Config{GitHubOwner: owner}); err != nil {
			return err
		}
	}

	result, err := scaffold.Generate(cmd.Context(), scaffold.Options{
		Name: name, Module: modulePath, Directory: options.directory,
		Framework: options.framework, Database: options.database, Queue: options.queue,
		Scheduler: options.scheduler, SkipTidy: options.skipTidy,
	})
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Created %s\n", result.Destination); err != nil {
		return err
	}

	if result.TidyErr != nil {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "go mod tidy failed: %v\nRun it manually in %s\n", result.TidyErr, result.Destination)

		return result.TidyErr
	}

	_, err = fmt.Fprintf(cmd.OutOrStdout(), "Next: cd %s && cp .env.example .env && make migrate && make run-server\n", result.Destination)

	return err
}

func runWizard(cmd *cobra.Command, options *newOptions, owner *string) error {
	fields := []huh.Field{
		huh.NewInput().Title("GitHub owner").Value(owner).Validate(validateOwner),
		huh.NewSelect[string]().Title("HTTP framework").Options(
			huh.NewOption("Pure Go (net/http)", "nethttp"),
			huh.NewOption("Fiber", "fiber"),
			huh.NewOption("Gin", "gin"),
			huh.NewOption("Echo", "echo"),
		).Value(&options.framework),
		huh.NewSelect[string]().Title("Database").Options(
			huh.NewOption("MySQL", "mysql"),
			huh.NewOption("PostgreSQL", "postgres"),
			huh.NewOption("SQLite", "sqlite"),
		).Value(&options.database),
		huh.NewSelect[string]().Title("Queue").Options(
			huh.NewOption("None (synchronous mail)", "none"),
			huh.NewOption("Database", "database"),
			huh.NewOption("Redis", "redis"),
		).Value(&options.queue),
	}

	form := huh.NewForm(huh.NewGroup(fields...)).WithAccessible(options.accessible)
	if err := form.RunWithContext(cmd.Context()); err != nil {
		return fmt.Errorf("run project wizard: %w", err)
	}

	if options.queue != "none" {
		form = huh.NewForm(huh.NewGroup(
			huh.NewConfirm().Title("Include the scheduler process?").Value(&options.scheduler),
		)).WithAccessible(options.accessible)
		if err := form.RunWithContext(cmd.Context()); err != nil {
			return fmt.Errorf("run scheduler prompt: %w", err)
		}
	} else {
		options.scheduler = false
	}

	return nil
}

func validateOwner(value string) error {
	if !ownerPattern.MatchString(strings.TrimSpace(value)) {
		return errors.New("GitHub owner must be 1-39 letters, digits, or hyphens and start with a letter or digit")
	}

	return nil
}

func stdinIsTerminal() bool {
	info, err := os.Stdin.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}
