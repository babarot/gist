package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	// Version is the version number
	Version = "unset"

	// BuildTag set during build to git tag, if any
	BuildTag = "unset"

	// BuildSHA is the git sha set during build
	BuildSHA = "unset"
)

// newRootCmd returns the root command
func newRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:                "gist",
		Short:              "A simple gist editor for CLI",
		SilenceErrors:      true,
		DisableSuggestions: false,
		Version:            fmt.Sprintf("%s (%s/%s)", Version, BuildTag, BuildSHA),
	}

	rootCmd.AddCommand(newNewCmd())
	rootCmd.AddCommand(newEditCmd())
	rootCmd.AddCommand(newOpenCmd())
	rootCmd.AddCommand(newDeleteCmd())
	return rootCmd
}

// Execute is the entrypoint of this cmd package
func Execute() error {
	// clilog.Env = "GIST_LOG"
	// clilog.Path = "GIST_LOG_PATH"
	// clilog.SetOutput()
	//
	// log.Printf("[INFO] pkg version: %s", Version)
	// log.Printf("[INFO] Go runtime version: %s", runtime.Version())
	// log.Printf("[INFO] Build tag/SHA: %s/%s", BuildTag, BuildSHA)
	// log.Printf("[INFO] CLI args: %#v", os.Args)
	//
	// defer log.Printf("[DEBUG] root command execution finished")

	return newRootCmd().Execute()
}
