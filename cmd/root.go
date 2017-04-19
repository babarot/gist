package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/b4b4r07/gist/config"
	"github.com/spf13/cobra"
)

const Version = "0.1.2"

var showVersion bool

var RootCmd = &cobra.Command{
	Use:           "gist",
	Short:         "gist editor",
	Long:          "gist - A simple gist editor for CLI",
	SilenceUsage:  true,
	SilenceErrors: true,
	Run: func(cmd *cobra.Command, args []string) {
		if showVersion {
			fmt.Printf("version %s/%s\n", Version, runtime.Version())
			return
		}
		cmd.Usage()
	},
}

func Execute() {
	err := RootCmd.Execute()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConf)
	RootCmd.Flags().BoolVarP(&showVersion, "version", "v", false, "show the version and exit")
}

func initConf() {
	dir, _ := config.GetDefaultDir()
	toml := filepath.Join(dir, "config.toml")

	err := config.Conf.LoadFile(toml)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error: %v", err)
		os.Exit(1)
	}
}
