package cli

import (
	"fmt"
	"runtime"

	"github.com/rifkifajarramadhani/go-craft/internal/configstore"
	"github.com/rifkifajarramadhani/go-craft/internal/scaffold"
	"github.com/spf13/cobra"
)

func NewRootCommand() *cobra.Command {
	store, err := configstore.New()
	if err != nil {
		store = configstore.NewAt("")
	}

	root := &cobra.Command{
		Use:           "go-craft",
		Short:         "Craft a clean-architecture Go service",
		SilenceUsage:  true,
		SilenceErrors: false,
	}
	root.AddCommand(
		newCommand(store),
		configCommand(store),
		completionCommand(),
		versionCommand(),
	)

	return root
}

func versionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print generator and template versions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "go-craft %s (template %s, %s/%s)\n",
				scaffold.GeneratorVersion, scaffold.TemplateVersion, runtime.GOOS, runtime.GOARCH)

			return err
		},
	}
}

func completionCommand() *cobra.Command {
	command := &cobra.Command{
		Use:                   "completion [bash|zsh|fish|powershell]",
		Short:                 "Generate a shell completion script",
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return cmd.Root().GenBashCompletionV2(cmd.OutOrStdout(), true)
			case "zsh":
				return cmd.Root().GenZshCompletion(cmd.OutOrStdout())
			case "fish":
				return cmd.Root().GenFishCompletion(cmd.OutOrStdout(), true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
			default:
				return fmt.Errorf("unsupported shell %q", args[0])
			}
		},
	}

	return command
}
