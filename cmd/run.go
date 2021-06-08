/*
Copyright © 2021 Drew S. Ortega <DrewSOrtega@pm.me>

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
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/spf13/cobra"

	lib "github.com/OSU-SOC/nagini/lib"
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run [log type] [command] [args...]",
	Short: "Parallelize log pull using filter from given command.",
	Long: `Parallelize log pull using filter from given command. Requires a command that accepts input from stdin, and produces output on stdout.

Example:
	nagini run -t 8 rdp grecidr 10.0.0.0/24
`,
	Args: cobra.MinimumNArgs(2), // 1 argument: script to run
	Run: func(cmd *cobra.Command, args []string) {

		// parse params and args
		startTime, endTime, resolvedOutDir, resolvedLogDir, logType, targetCommand, targetCommandArgs := parseRunParams(cmd, args[0], args[1:])

		// list params
		cmd.Printf("Zeek Log Directory:\t%s\n", logDir)
		cmd.Printf("Log Type:\t\t%s\n", logType)
		cmd.Printf("Date Range:\t\t%s - %s\n", startTime.Format(lib.TimeFormatHuman), endTime.Format(lib.TimeFormatHuman))
		cmd.Printf("Command to run:\t\t%s %s\n", targetCommand, strings.Join(targetCommandArgs, " "))
		cmd.Printf("Threads:\t\t%d\n", threads)
		cmd.Printf("Output Directory:\t%s\n\n", resolvedOutDir)

		// prompt if continue
		if !lib.WaitForConfirm(cmd) {
			// if start is no, do not continue
			return
		}

		// The response was yes- continue.

		// create the output directory.
		e := lib.TryCreateDir(resolvedOutDir, true)
		if e != nil {
			cmd.PrintErrln(e)
		} else {
			debugLog.Printf("created dir %s\n", resolvedOutDir)
		}

		// set parallel routine thread limit
		runtime.GOMAXPROCS(threads)

		// time iterators
		curDate := startTime.Truncate(24 * time.Hour) // start at this date, at 00:00:00
		curTime := startTime                          // start at this hour

		// progress bars init
		dayCount := int(endTime.Round(time.Hour*24).Sub(startTime.Truncate(time.Hour*24)).Hours() / 24.0) // calculate total number of days
		barPool, dayBar, taskBar := lib.InitBars(dayCount, taskCount, debugLog)

		// holds wait interface for all routines to finish.
		var wgAll sync.WaitGroup

		// for each date
		for curDate.Before(endTime) || curDate.Equal(endTime) {
			// holds wait interface for all routines of this particular day.
			var wgDate sync.WaitGroup
			var tempFiles []string
			// for each hour of that date, excluding the last date where we may end early.
			for curTime.Before(curDate.AddDate(0, 0, 1)) && (curTime.Before(endTime) || curTime.Equal(endTime)) {
				// find all input files that match this hour
				inputFileGlob := fmt.Sprintf("%s/%04d-%02d-%02d/%s.%02d*", resolvedLogDir, curTime.Year(), curTime.Month(), curTime.Day(), logType, curTime.Hour())
				logFileMatches, e := filepath.Glob(inputFileGlob)
				if e != nil {
					debugLog.Printf("ERROR (%s): %s\n", curTime.Format(lib.TimeFormatHuman), e)
					continue
				}
				taskCount += len(logFileMatches) // set total number of found log files, plus one for the concatenation step.
				taskBar.SetTotal(taskCount)      // set new total on bar to include found log files
				taskBar.Update()

				// for every found log file, run the script.
				for _, logFile := range logFileMatches {
					outputFileTemp := filepath.Join(
						outputDir,
						curTime.Format(lib.TimeFormatDateNum)+filepath.Base(logFile)+".json",
					)
					tempFiles = append(tempFiles, outputFileTemp)
					//runScript(targetCommand, logFile, outputFileTemp, curTime, &wgDate, taskBar)
				}
				curTime = curTime.Add(time.Hour)
			}

			// wait for all date's to finish each log and then for them to concat into a single file.
			wgAll.Add(1)
			go outputDateParallel(logType, tempFiles, debugLog, curDate, &wgDate, &wgAll, dayBar)

			// iterate to next date
			curDate = curDate.AddDate(0, 0, 1)
		}

		// wait for each day's go routine to finish. When done, exit!
		debugLog.Println("All routines queued. Waiting for them to finish.")

		wgAll.Wait()
		barPool.Stop()

		cmd.Printf("\nComplete. Output: %s\n", outputDir)
		return
	},
}

func init() {
	rootCmd.AddCommand(runCmd)

	// init vars
	taskCount = 0

	// Add flags

	// time range to parse
	runCmd.PersistentFlags().StringVarP(
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

	runCmd.PersistentFlags().StringVarP(&outputDir, "outdir", "o",
		defaultPath,
		"filtered logs output directory",
	)

}

// takes args and params, does error checking, and then produces useful variables.
func parseRunParams(cmd *cobra.Command, logTypeArg string, commandToRun []string) (startTime time.Time, endTime time.Time, resolvedOutDir string, resolvedLogDir string, logType string, execPath string, execArgs []string) {
	startTime, endTime, resolvedOutDir, resolvedLogDir, logType = lib.ParseSharedArgs(cmd, timeRange, logDir, outputDir, logTypeArg)

	lookInPath := false
	// try to resolve script, see if it exists.
	localExecPath, e1 := filepath.Abs(commandToRun[0])
	_, e2 := os.Stat(localExecPath)
	if e1 != nil || e2 != nil {
		// could not find local file, so look for it in path
		lookInPath = true
	} else {
		// found local file, see if it is executable
		localExecPath, e := exec.LookPath(localExecPath)
		if e != nil {
			// the local file is not executable, so lets go search path.
			lookInPath = true
		} else {
			// local file exists and is executable. use it.
			execPath = localExecPath
		}
	}

	// if we failed at all to look for a local file, look in path.
	if lookInPath {
		execPath, e1 = exec.LookPath(commandToRun[0])
		if e1 != nil {
			// no local file or file in path that is executable. Error and exit.
			cmd.PrintErrf("error: could not find an executable '%s'. Make sure it exists and is marked as executable.\n", commandToRun[0])
			os.Exit(1)
		}
	}

	execArgs = commandToRun[1:]
	return
}
