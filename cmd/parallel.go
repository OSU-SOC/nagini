/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	lib "github.com/OSU-SOC/nagini/lib"
	wmenu "gopkg.in/dixonwille/wmenu.v4"
)

// args
var threads uint     // number of threads to run
var timeRange string // string format of time range to go over
var outputDir string // directory to output logs
var logDir string    // directory containing all zeek logs
var logType string   // type of log (rdp, ftp, etc)

// calculated start time and end time values
var startTime time.Time
var endTime time.Time

// parallelCmd represents the parallel command
var parallelCmd = &cobra.Command{
	Use:   "parallel [log type] [shell script]",
	Short: "Legacy parallel for backwards compatibility with old parallel.py.",
	Long: `Legacy parallel for backwards compatibility with old parallel.py.

Example:
	nagini parallel -t 8 rdp my_script.py 

where my_script.py has the following required syntax:
	./my_script.py [input file] [output file]
`,
	Args: cobra.ExactArgs(2), // 1 argument: script to run
	Run: func(cmd *cobra.Command, args []string) {
		logType = args[0]
		// build time range timestamps
		var dateStrings = strings.Split(timeRange, "-")
		startTime, startErr := time.Parse(lib.TimeFormatShort, dateStrings[0])
		endTime, endErr := time.Parse(lib.TimeFormatShort, dateStrings[1])

		// if failed to generate timestamp values, error out
		if startErr != nil || endErr != nil {
			cmd.PrintErrln("error: Provided dates malformed. Please provide dates in the following format: YYYY/MM/DD:HH-YYYY/MM/DD:HH")
			return
		}

		// try to resolve output directory, see if it is valid input.
		resolvedDir, e := filepath.Abs(outputDir)
		if e != nil {
			cmd.PrintErrln("error: could not resolve relative path in user provided input.")
			return
		}

		// try to resolve zeek log dir and see if exists and is real dir.
		resolvedLogDir, e := filepath.Abs(logDir)
		if e != nil {
			cmd.PrintErrln("error: could not resolve relative path in user provided input.")
			return
		}
		logDirInfo, e := os.Stat(resolvedLogDir)
		if os.IsNotExist(e) || !logDirInfo.IsDir() {
			cmd.PrintErrf("error: invalid Zeek log directory %s, either does not exist or is not a directory.\n", resolvedLogDir)
			return
		}

		// try to resolve script, see if it exists.
		scriptPath, e := filepath.Abs(args[1])
		if e != nil {
			cmd.PrintErrln("error: could not resolve relative path in user provided input.")
			return
		}

		// check to see if script file exists.
		_, e = os.Stat(scriptPath)
		if os.IsNotExist(e) {
			cmd.PrintErrf("error: script '%s' does not exist.\n", scriptPath)
			return
		}

		// list params

		cmd.Printf("Zeek Log Directory:\t%s\n", logDir)
		cmd.Printf("Log Type:\t\t%s\n", logType)
		cmd.Printf("Date Range:\t\t%s - %s\n", startTime.Format(lib.TimeFormatHuman), endTime.Format(lib.TimeFormatHuman))
		cmd.Printf("Script to Run:\t\t%s\n", scriptPath)
		cmd.Printf("Threads:\t\t%d\n", threads)
		cmd.Printf("Output Directory:\t%s\n\n", resolvedDir)

		// prompt if continue
		var start int
		startMenu := wmenu.NewMenu("Continue?")
		startMenu.IsYesNo(0)
		startMenu.LoopOnInvalid()
		startMenu.Action(func(opts []wmenu.Opt) error {
			start = opts[0].ID
			return nil
		})
		e = startMenu.Run()
		if e != nil {
			cmd.PrintErrln(e)
			return
		}

		// if start is no, do not continue
		if start == 1 {
			return
		}
		e = lib.TryCreateDir(resolvedDir)
		if e != nil {
			cmd.PrintErrln(e)
		} else {
			cmd.Printf("created dir %s\n", resolvedDir)
		}

		// Continue, so lets start parsing
		curDate := startTime
		for curDate.Before(endTime) || curDate.Equal(endTime) {
			cmd.Printf("%s/%04d-%02d-%02d/%s.%02d\n", resolvedLogDir, curDate.Year(), curDate.Month(), curDate.Day(), logType, curDate.Hour())
			curDate = curDate.Add(time.Hour)
		}
	},
}

func init() {
	rootCmd.AddCommand(parallelCmd)

	// Add flags

	// threads
	parallelCmd.PersistentFlags().UintVarP(&threads, "threads", "t", 1, "Number of threads to run in parallel")

	// time range to parse
	parallelCmd.PersistentFlags().StringVarP(
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

	parallelCmd.PersistentFlags().StringVarP(&outputDir, "outdir", "o",
		defaultPath,
		"filtered logs output directory",
	)

	parallelCmd.PersistentFlags().StringVarP(&logDir, "logdir", "i",
		filepath.Join("/", "opt", "zeek", "logs"),
		"Zeek log directory",
	)
}
