package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/b4b4r07/gist/cli"
	"github.com/spf13/cobra"
)

const Version = "0.1.5"

var (
	showVersion bool
	cacheClear  bool
)

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
		if cacheClear {
			cli.NewCache().Clear()
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
	initConf()
	RootCmd.Flags().BoolVarP(&showVersion, "version", "v", false, "show the version and exit")
	RootCmd.Flags().BoolVarP(&cacheClear, "cache-clear", "c", false, "clear cache")
}

func initConf() {
	dir, _ := cli.GetDefaultDir()
	toml := filepath.Join(dir, "config.toml")

	err := cli.Conf.LoadFile(toml)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
