package cmd

import (
	"fmt"
	"os"

	"github.com/fairy-pitta/portree/internal/config"
	"github.com/fairy-pitta/portree/internal/git"
	"github.com/fairy-pitta/portree/internal/logging"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:         "init",
	Short:       "Initialize a .portree.toml configuration file",
	Long:        "Creates a default .portree.toml in the current git repository root.",
	Annotations: map[string]string{"skipRepoDetection": "true"},
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting current directory: %w", err)
		}

		root, err := git.FindRepoRoot(cwd)
		if err != nil {
			return fmt.Errorf("not inside a git repository")
		}

		path, err := config.Init(root)
		if err != nil {
			return err
		}

		fmt.Printf("Created %s in %s\n", config.FileName, root)

		// The state directory holds the dev CA private key, logs and PIDs, so
		// keep it out of the repository. Failing to ignore it is not worth
		// aborting init over, but the user should hear about it.
		added, err := config.EnsureStateIgnored(root)
		switch {
		case err != nil:
			logging.Warn("could not update .gitignore: %v", err)
			logging.Warn("add %s/ yourself: it holds runtime state and the dev CA private key", config.StateDirName)
		case added:
			fmt.Printf("Added %s/ to .gitignore\n", config.StateDirName)
		}

		fmt.Printf("Edit the file to configure your services: %s\n", path)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
