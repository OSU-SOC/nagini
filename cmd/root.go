package cmd

import (
	"io"
	"log"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// args
var threads int      // number of threads to run
var verbose bool     // verbose
var timeRange string // string format of time range to go over
var outputDir string // directory to output logs
var logDir string    // directory containing all zeek logs
var singleFile bool  // holds whether or not to concat into one file.

// calculated start time and end time values
var startTime time.Time
var endTime time.Time

// global vars
var debugLog *log.Logger
var runtimeConfig *viper.Viper

// other
var taskCount int // hold count of goroutines to wait on

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "nagini",
	Short: "Pull and filter logs to a subset for easier parsing.",
	Long:  `Pull and filter logs to a subset for easier parsing.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// set up logger based on verbosity
		if verbose == true {
			debugLog = log.New(os.Stdout, "", log.LstdFlags)
		} else {
			debugLog = log.New(io.Discard, "", 0)
		}

	},
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

}
