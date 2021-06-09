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
	"compress/gzip"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	pb "github.com/cheggaaa/pb"
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

		// parse the given logs based on the runCommand handler.
		lib.ParseLogs(cmd,
			func(logFile string, outputFile string, curTime time.Time, wgDate *sync.WaitGroup, taskBar *pb.ProgressBar) {
				runCommand(targetCommand, targetCommandArgs, logFile, outputFile, curTime, wgDate, taskBar)
			},
			debugLog, startTime, endTime, logType, resolvedLogDir, resolvedOutDir, threads, singleFile)

		cmd.Printf("\nComplete. Output: %s\n", outputDir)
		return
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
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

// takes input file, script, and output file, and runs script in parallel, syncing given wait group.
func runCommand(cmdPath string, cmdArgs []string, logFile string, outputFile string, curTime time.Time, wgDate *sync.WaitGroup, taskBar *pb.ProgressBar) {
	wgDate.Add(1)

	// start concurrent method. Look through this log file, write to temp file, and then let
	// the date know it is done.
	go func(logFile string, outputFile string, wgDate *sync.WaitGroup, taskBar *pb.ProgressBar) {
		defer wgDate.Done()
		defer taskBar.Increment()

		debugLog.Printf("queued: %s -> %s\n", logFile, outputFile)

		// open input file for reading as compressed
		cmdInputCompressed, fileReadErr := os.Open(logFile)
		if fileReadErr != nil {
			fmt.Printf("ERROR (%s): %s\n", curTime.Format(lib.TimeFormatHuman), fileReadErr)
			return
		}
		defer cmdInputCompressed.Close()

		// open input file for reading as compressed
		cmdInput, fileReadZipErr := gzip.NewReader(cmdInputCompressed)
		if fileReadZipErr != nil {
			fmt.Printf("ERROR (%s): %s\n", curTime.Format(lib.TimeFormatHuman), fileReadErr)
			return
		}
		defer cmdInput.Close()

		// open output file for writing
		cmdOutput, fileWriteErr := os.Create(outputFile)
		if fileWriteErr != nil {
			fmt.Printf("ERROR (%s): %s\n", curTime.Format(lib.TimeFormatHuman), fileWriteErr)
			return
		}
		defer cmdOutput.Close()

		// run script, which should handle the file writing itself currently.
		cmdContext := exec.Command(cmdPath, cmdArgs...)
		cmdContext.Stdin = cmdInput
		cmdContext.Stdout = cmdOutput

		runErr := cmdContext.Run()
		if runErr != nil {
			debugLog.Printf("ERROR (%s): %s\n", curTime.Format(lib.TimeFormatHuman), runErr)
		}
	}(logFile, outputFile, wgDate, taskBar)
}
