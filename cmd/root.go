package cmd

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	lib "github.com/OSU-SOC/nagini/lib"
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
var noConfirm bool   // if set, skips continue prompt.
var writeStdout bool // if set, writes to Stdout instead of the output directory.

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
			debugLog = log.New(os.Stderr, "", log.LstdFlags)
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

	rootCmd.SetOut(os.Stderr)
	// read flags
	// Set up global configuration path.
	globalConfig := lib.ReadGlobalConfig()

	// threads
	rootCmd.PersistentFlags().IntVarP(&threads, "threads", "t", globalConfig.GetInt("default_thread_count"), "Number of threads to run in parallel")

	// default zeek dir
	rootCmd.PersistentFlags().StringVarP(&logDir, "logdir", "i",
		globalConfig.GetString("zeek_log_dir"),
		"Zeek log directory",
	)

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")

	rootCmd.PersistentFlags().BoolVarP(&singleFile, "concat", "c",
		globalConfig.GetBool("concat_by_default"),
		"concat all output to one file, rather than files for each date.",
	)
	rootCmd.PersistentFlags().BoolVarP(&noConfirm, "noconfirm", "N",
		false,
		"Skip confirmation and begin operation.",
	)
	rootCmd.PersistentFlags().BoolVarP(&writeStdout, "stdout", "S",
		false,
		"Do not write to output directory, instead write to STDOUT.",
	)

	// time range to parse
	rootCmd.PersistentFlags().StringVarP(
		&timeRange, "timerange", "r",
		fmt.Sprintf( // write range of last 24 hours
			"%s-%s",
			time.Now().AddDate(0, 0, -1).Format(lib.TimeFormatShort), // yesterday at this time
			time.Now().Format(lib.TimeFormatShort)),                  // right now
		"time-range (local time). unspecified: last 24 hours. Format: YYYY/MM/DD:HH-YYYY/MM/DD:HH",
	)

	// default path for log storage is ./output-DATE
	// uses this if no path specified.
	defaultPath, e := filepath.Abs("./output-" + time.Now().Format(lib.TimeFormatLongNum))
	if e != nil {
		panic("fatal error: could not resolve relative path")
	}

	rootCmd.PersistentFlags().StringVarP(&outputDir, "outdir", "o",
		defaultPath,
		"filtered logs output directory",
	)
}
