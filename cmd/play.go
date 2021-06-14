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
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	pb "github.com/cheggaaa/pb"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	lib "github.com/OSU-SOC/nagini/lib"
)

// runCmd represents the run command
var playCmd = &cobra.Command{
	Use:   "play [config file]",
	Short: "Parallelize log pull using settings from the config file.",
	Long: `Parallelize log pull using settings from the config file. Requires a command that accepts input from stdin, and produces output on stdout.

Example:
	nagini play my_config.yaml

where my config.yaml is something similar to:
    exec: my_script.sh
	output: my_dir
	threads: 4
	log_type: dns
	concat: true
`,
	Args: cobra.MinimumNArgs(2), // 1 argument: script to run
	Run: func(cmd *cobra.Command, args []string) {
		// parse params and args
		resolvedStartTime, resolvedEndTime, resolvedOutDir, resolvedLogDir, resolvedLogType, targetCommand, targetCommandArgs := parseRunParams(cmd, args[0], args[1:])

		// list params
		cmd.Printf("Zeek Log Directory:\t%s\n", config.ZeekLogDir)
		cmd.Printf("Log Type:\t\t%s\n", resolvedLogType)
		cmd.Printf("Date Range:\t\t%s - %s\n", resolvedStartTime.Format(lib.TimeFormatHuman), resolvedEndTime.Format(lib.TimeFormatHuman))
		cmd.Printf("Command to run:\t\t%s %s\n", targetCommand, strings.Join(targetCommandArgs, " "))
		cmd.Printf("Threads:\t\t%d\n", config.Threads)
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
			debugLog, resolvedStartTime, resolvedEndTime, resolvedLogType, resolvedLogDir, resolvedOutDir, config.Threads, config.Concat, config.Stdout)

		cmd.Printf("\nComplete. Output: %s\n", resolvedOutDir)
		return
	},
}

func init() {
	rootCmd.AddCommand(runCmd)
}

// takes args and params, does error checking, and then produces useful variables.
func parsePlayParams(cmd *cobra.Command, configFile string) (startTime time.Time, endTime time.Time, resolvedOutDir string, resolvedLogDir string, logType string, execPath string, execArgs []string) {
	startTime, endTime, resolvedOutDir, resolvedLogDir, logType = lib.ParseSharedArgs(cmd, config.RawTimeRange, config.ZeekLogDir, config.OutputDir, config.LogType)

	// TODO
	runtimeConfig, err := readRuntimeConfig(configFile)

	return
}

func readRuntimeConfig(configFile string) (runtimeConfig *viper.Viper, err error) {
	configFileReader, err := os.Open(configFile)
	if err != nil {
		return nil, err
	}

	// default path for log storage is ./output-DATE
	// uses this if no path specified.
	defaultPath, e := filepath.Abs("./output-" + time.Now().Format(lib.TimeFormatLongNum))
	if e != nil {
		return nil, e
	}
	runtimeConfig = viper.New()

	// copy global config into runtime config. Will be overrode if exists in runtime config.
	runtimeConfig.SetDefault("threads", globalConfig.GetInt("default_thread_count"))
	runtimeConfig.SetDefault("logdir", globalConfig.GetString("zeek_log_dir"))
	runtimeConfig.SetDefault("concat", globalConfig.GetBool("concat_by_default"))
	runtimeConfig.SetDefault("outdir", defaultPath)

	// read the runtime config.
	e = runtimeConfig.ReadConfig(configFileReader)
	if e != nil {
		return nil, e
	}

	// check for presence of fields with no default values:
	if !runtimeConfig.IsSet("command") {
		return nil, errors.New("config value 'command' not set")
	}
	if !runtimeConfig.IsSet("type") {
		return nil, errors.New("config value 'type' not set. Ex. dns, rdp, smtp")
	}

	// TODO: allow other types of date inputs
	if runtimeConfig.IsSet("time_range") {
		if !runtimeConfig.IsSet("time_range.start_time") ||
			!runtimeConfig.IsSet("time_range.end_time") {
			return nil, errors.New("config value 'time_range' provided but sub fields 'start_time' and/or 'end_time' are not provided.")
		} else {
			// TODO: parse dates
		}

	} else {
		return nil, errors.New("No date range provided.")
	}

	return runtimeConfig, nil
}
