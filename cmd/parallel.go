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
	"sync"
	"time"

	pb "github.com/cheggaaa/pb"
	"github.com/spf13/cobra"

	lib "github.com/OSU-SOC/nagini/lib"
)

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
		startTime, endTime, resolvedOutDir, resolvedLogDir, logType, scriptPath := parseParallelParams(cmd, args[0], args[1])

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

		// parse the given logs based on the runScript handler.
		lib.ParseLogs(cmd,
			func(logFile string, outputFile string, curTime time.Time, wgDate *sync.WaitGroup, taskBar *pb.ProgressBar) {
				runScript(scriptPath, logFile, outputFile, curTime, wgDate, taskBar)
			},
			debugLog, startTime, endTime, logType, resolvedLogDir, resolvedOutDir, threads, singleFile,
		)

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
func parseParallelParams(cmd *cobra.Command, logTypeArg string, scriptPathArg string) (startTime time.Time, endTime time.Time, resolvedOutDir string, resolvedLogDir string, logType string, scriptPath string) {
	startTime, endTime, resolvedOutDir, resolvedLogDir, logType = lib.ParseSharedArgs(cmd, timeRange, logDir, outputDir, logTypeArg)

	// try to resolve script, see if it exists.
	scriptPath, e := filepath.Abs(scriptPathArg)
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

	return
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
		taskBar.Increment()
	}(logFile, outputFile, wgDate, taskBar)
}
