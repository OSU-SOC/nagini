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
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	pb "github.com/cheggaaa/pb"
	"github.com/spf13/cobra"

	lib "github.com/OSU-SOC/nagini/lib"
)

// args
var timeRange string // string format of time range to go over
var outputDir string // directory to output logs
var logDir string    // directory containing all zeek logs

// calculated start time and end time values
var startTime time.Time
var endTime time.Time

// other
var taskCount int // hold count of goroutines to wait on

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

		// parse params and args
		startTime, endTime, resolvedOutDir, resolvedLogDir, logType, scriptPath := parseParams(cmd, args[0], args[1])

		// list params
		cmd.Printf("Zeek Log Directory:\t%s\n", logDir)
		cmd.Printf("Log Type:\t\t%s\n", logType)
		cmd.Printf("Date Range:\t\t%s - %s\n", startTime.Format(lib.TimeFormatHuman), endTime.Format(lib.TimeFormatHuman))
		cmd.Printf("Script to Run:\t\t%s\n", scriptPath)
		cmd.Printf("Threads:\t\t%d\n", threads)
		cmd.Printf("Output Directory:\t%s\n\n", resolvedOutDir)

		// prompt if continue
		if !lib.WaitForConfirm(cmd) {
			// if start is no, do not continue
			return
		}

		// The response was yes- continue.

		// create the output directory.
		e := lib.TryCreateDir(resolvedOutDir)
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
				taskCount += len(logFileMatches) // set total number of found log files, plus one for the concatination step.
				taskBar.SetTotal(taskCount)      // set new total on bar to include found log files
				taskBar.Update()

				// for every found log file, run the script.
				for _, logFile := range logFileMatches {
					outputFileTemp := filepath.Join(
						outputDir,
						curTime.Format(lib.TimeFormatDateNum)+filepath.Base(logFile)+".json",
					)
					tempFiles = append(tempFiles, outputFileTemp)
					runScript(scriptPath, logFile, outputFileTemp, curTime, &wgDate, taskBar)
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
	rootCmd.AddCommand(parallelCmd)

	// init vars
	taskCount = 0

	// Add flags

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

}

// takes args and params, does error checking, and then produces useful variables.
func parseParams(cmd *cobra.Command, logTypeArg string, scriptPathArg string) (startTime time.Time, endTime time.Time, resolvedOutDir string, resolvedLogDir string, logType string, scriptPath string) {
	// build time range timestamps
	var dateStrings = strings.Split(timeRange, "-")
	startTime, startErr := time.Parse(lib.TimeFormatShort, dateStrings[0])
	endTime, endErr := time.Parse(lib.TimeFormatShort, dateStrings[1])

	// if failed to generate timestamp values, error out
	if startErr != nil || endErr != nil {
		cmd.PrintErrln("error: Provided dates malformed. Please provide dates in the following format: YYYY/MM/DD:HH-YYYY/MM/DD:HH")
		os.Exit(1)
	}

	// try to resolve output directory, see if it is valid input.
	resolvedOutDir, e := filepath.Abs(outputDir)
	if e != nil {
		cmd.PrintErrln("error: could not resolve relative path in user provided input.")
		os.Exit(1)
	}

	// try to resolve zeek log dir and see if exists and is real dir.
	resolvedLogDir, e = filepath.Abs(logDir)
	if e != nil {
		cmd.PrintErrln("error: could not resolve relative path in user provided input.")
		os.Exit(1)
	}
	logDirInfo, e := os.Stat(resolvedLogDir)
	if os.IsNotExist(e) || !logDirInfo.IsDir() {
		cmd.PrintErrf("error: invalid Zeek log directory %s, either does not exist or is not a directory.\n", resolvedLogDir)
		os.Exit(1)
	}

	// try to resolve script, see if it exists.
	scriptPath, e = filepath.Abs(scriptPathArg)
	if e != nil {
		cmd.PrintErrln("error: could not resolve relative path in user provided input.")
		os.Exit(1)
	}

	// check to see if script file exists.
	_, e = os.Stat(scriptPath)
	if os.IsNotExist(e) {
		cmd.PrintErrf("error: script '%s' does not exist.\n", scriptPath)
		os.Exit(1)
	}

	_, e = exec.LookPath(scriptPath)
	if e != nil {
		cmd.PrintErrf("error: script '%s' exists but is not marked as an executable.\n", scriptPath)
		os.Exit(1)
	}

	// TODO: add logType verification
	logType = logTypeArg

	return
}

// Waits until the given sync group is done. When it finishes, concats all files together of that particular date, and then lets the global sync group know it has finished.
func outputDateParallel(logType string, inputFiles []string, logger *log.Logger, curDate time.Time, wgDate *sync.WaitGroup, wgAll *sync.WaitGroup, bar *pb.ProgressBar) {
	outputFile := filepath.Join(
		outputDir,
		fmt.Sprintf("%s-%04d-%02d-%02d.json", logType, curDate.Year(), curDate.Month(), curDate.Day()),
	)

	// Wait for all log files for this date to finish.
	wgDate.Wait()
	defer wgAll.Done()
	defer bar.Increment()

	logger.Printf("All logs for %s finished. Concatinating into '%s'\n", curDate.Format(lib.TimeFormatDate), outputFile)

	// keep track of concat failures to alert the program.
	failure := false

	// if no input files, ignore.
	if len(inputFiles) == 0 {
		logger.Printf("WARN: No matches for date %s. Skipping.\n", curDate.Format(lib.TimeFormatDate))
	} else {
		e := lib.ConcatFiles(logger, inputFiles, outputFile, true)
		if e != nil {
			logger.Println("ERROR: ", e)
			failure = true
		}
	}

	// print whether or not we failed to concat the files together.
	if failure {
		logger.Printf("FAIL: %s\n", curDate.Format(lib.TimeFormatDate))
	} else {
		logger.Printf("SUCCESS: %s\n", curDate.Format(lib.TimeFormatDate))
	}
}

// takes input file, script, and output file, and runs script in parallel, syncing given wait group.
func runScript(scriptPath string, logFile string, outputFile string, curTime time.Time, wgDate *sync.WaitGroup, taskBar *pb.ProgressBar) {
	wgDate.Add(1)

	// start concurrent method. Look through this log file, write to temp file, and then let
	// the date know it is done.
	go func(logFile string, outputFile string, wgDate *sync.WaitGroup, taskBar *pb.ProgressBar) {
		debugLog.Printf("queued: %s -> %s\n", logFile, outputFile)

		// run script, which should handle the file writing itself currently.
		runErr := exec.Command(scriptPath, logFile, outputFile).Run()
		if runErr != nil {
			debugLog.Printf("ERROR (%s): %s\n", curTime.Format(lib.TimeFormatHuman), runErr)
		}
		defer wgDate.Done()
		defer taskBar.Increment()
	}(logFile, outputFile, wgDate, taskBar)
}
