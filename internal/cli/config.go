package cli

import (
	"fmt"
	"strings"

	"github.com/rifkifajarramadhani/go-craft/internal/configstore"
	"github.com/spf13/cobra"
)

func configCommand(store *configstore.Store) *cobra.Command {
	command := &cobra.Command{Use: "config", Short: "Manage go-craft configuration"}
	command.AddCommand(configGetCommand(store), configSetCommand(store))

	return command
}

func configGetCommand(store *configstore.Store) *cobra.Command {
	return &cobra.Command{
		Use:       "get github-owner",
		Short:     "Print the configured default GitHub owner",
		ValidArgs: []string{"github-owner"},
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := store.Load()
			if err != nil {
				return err
			}

			_, err = fmt.Fprintln(cmd.OutOrStdout(), cfg.GitHubOwner)

			return err
		},
	}
}

func configSetCommand(store *configstore.Store) *cobra.Command {
	return &cobra.Command{
		Use:   "set github-owner <owner>",
		Short: "Set the default GitHub owner used in generated module paths",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if args[0] != "github-owner" {
				return fmt.Errorf("unsupported configuration key %q", args[0])
			}

			owner := strings.TrimSpace(args[1])
			if err := validateOwner(owner); err != nil {
				return err
			}

			if err := store.Save(configstore.Config{GitHubOwner: owner}); err != nil {
				return err
			}

			_, err := fmt.Fprintf(cmd.OutOrStdout(), "GitHub owner set to %s\n", owner)

			return err
		},
	}
}
