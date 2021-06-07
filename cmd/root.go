package cmd

import (
	"io"
	"log"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

// args
var threads int // number of threads to run
var verbose int // verbose

// global vars
var debugLog *log.Logger

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "nagini",
	Short: "Pull and filter logs to a subset for easier parsing.",
	Long:  `Pull and filter logs to a subset for easier parsing.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	// threads
	rootCmd.PersistentFlags().IntVarP(&threads, "threads", "t", 8, "Number of threads to run in parallel")

	// default zeek dir
	rootCmd.PersistentFlags().StringVarP(&logDir, "logdir", "i",
		filepath.Join("/", "data", "zeek", "logs"),
		"Zeek log directory",
	)

	rootCmd.PersistentFlags().CountVarP(&verbose, "verbose", "v", "enable verbose logging")
	// set up logger based on verbosity
	if verbose > 0 {
		debugLog = log.New(os.Stdout, "", log.LstdFlags)
	} else {
		debugLog = log.New(io.Discard, "", 0)
	}
}
