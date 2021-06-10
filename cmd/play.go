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
func parsePlayParams(cmd *cobra.Command, configFile string) (startTime time.Time, endTime time.Time, resolvedOutDir string, resolvedLogDir string, logType string, execPath string, execArgs []string) {
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

func readRuntimeConfig(globalConfig *viper.Viper, configFile string) (runtimeConfig *viper.Viper, err error) {
	configFileReader, err := os.Open(configFile)
	if err != nil {
		return nil, err
	}

	// default path for log storage is ./output-DATE
	// uses this if no path specified.
	defaultPath, e := filepath.Abs("./output-" + time.Now().Format(lib.TimeFormatLongNum))
	if e != nil {
		panic("fatal error: could not resolve relative path")
	}
	runtimeConfig = viper.New()

	// copy global config into runtime config. Will be overrode if exists in runtime config.
	runtimeConfig.SetDefault("threads", globalConfig.GetInt("default_thread_count"))
	runtimeConfig.SetDefault("logdir", globalConfig.GetString("zeek_log_dir"))
	runtimeConfig.SetDefault("concat", globalConfig.GetBool("concat_by_default"))
	runtimeConfig.SetDefault("outdir", defaultPath)

	// read the runtime config.
	runtimeConfig.ReadConfig(configFileReader)

	// set default vals for config generation
	globalConfig.SetDefault("default_thread_count", 8)
	globalConfig.SetDefault("zeek_log_dir", "/data/zeek/logs")
	globalConfig.SetDefault("concat_by_default", false)

	readConfig := true
	for readConfig {
		readConfig = false
		// Try ingesting config from one of the config paths.
		if err := globalConfig.ReadInConfig(); err != nil {
			if _, ok := err.(viper.ConfigFileNotFoundError); ok {
				fmt.Println("WARN: could not find a config file. Trying to write a default to /etc/nagini/config.yaml")
				// no config file exists.

				// first, try to see if we have write acess to /etc/nagini.
				err = TryCreateDir("/etc/nagini", false)
				if err == nil {
					// we have write access, create log file.
					err = viper.WriteConfigAs("/etc/nagini/config.yaml")
					if err == nil {
						fmt.Println("WARN: Successfully created log file at /etc/nagini/config.yaml")
						readConfig = true
						continue
					}
				}
				fmt.Println("WARN: could not read or write /etc/nagini/config.yaml, trying home directory ~/.config/nagini/config.yaml")
				// next, try to write to home directory if we don't have correct perms. Make sure config dir and nagini exists.
				homedir, err0 := os.UserHomeDir()
				err1 := TryCreateDir(homedir+"/.config", false)
				err2 := TryCreateDir(homedir+"/.config/nagini", false)
				if err0 != nil || err1 != nil || err2 != nil {
					panic(fmt.Errorf("No config file present, and failed to write a default. Please manually add one to /etc/nagini or ~/.config/nagini: %s %s", err1, err2))
				} else {
					err = viper.WriteConfigAs(homedir + "/.config/nagini/config.yaml")
					if err != nil {
						panic(fmt.Errorf("No config file present, and failed to write a default. Please manually add one to /etc/nagini or ~/.config/nagini: %s", err))
					} else {
						fmt.Printf("WARN: created a new config file at %s/.config/nagini/config.yaml\n", homedir)
						readConfig = true
					}
				}
			} else {
				// Config file was found but another error was produced
				panic(fmt.Errorf("Unexpected Error: %s", err))
			}
		}
	}
	return globalConfig
}
