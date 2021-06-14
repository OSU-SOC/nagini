package lib

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// The DataSource struct represents fields for an individual data source
// found in the config YAML file. It represents an individual log pull
// set, which will be stored in {ProjectName}/{Name}, unless ManualPath
// is specified.
// It will use Threads as the number of threads on the system to pull data
// with.
type DataSource struct {
	Name    string // name
	Threads int    // threads

	// one of: use specified log-path OR specify
	ManualPath string `yaml:"manual_path"` // manual_path
	Type       string `yaml:"log_type"`    //log_type
}

type Config struct {
	Verbose      bool
	Concat       bool
	Threads      int
	RawTimeRange string
	StartTime    time.Time
	EndTime      time.Time
	LogType      string
	ZeekLogDir   string
	OutputDir    string
	NoConfirm    bool
	Stdout       bool
}

// parses and verifies arguments that are global to the root command.
func ParseSharedArgs(cmd *cobra.Command, timeRange string, logDir string, outputDir string, logTypeArg string) (startTime time.Time, endTime time.Time, resolvedOutDir string, resolvedLogDir string, logType string) {
	// build time range timestamps
	var dateStrings = strings.Split(timeRange, "-")
	startTime, startErr := time.Parse(TimeFormatShort, dateStrings[0])
	endTime, endErr := time.Parse(TimeFormatShort, dateStrings[1])

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

	// TODO: add logType verification
	logType = logTypeArg

	return
}

// takes a global config from /etc/nagini or ~/.config/nagini, reads in vars that are present,
// and passes them as a viper config.
func ReadGlobalConfig() (globalConfig *viper.Viper) {
	globalConfig = viper.New()
	globalConfig.SetConfigName("config")
	globalConfig.SetConfigType("yaml")
	// Config paths in order of priority.
	globalConfig.AddConfigPath("/etc/nagini/")
	globalConfig.AddConfigPath("$HOME/.config/nagini/")

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
					err = globalConfig.WriteConfigAs("/etc/nagini/config.yaml")
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
					err = globalConfig.WriteConfigAs(homedir + "/.config/nagini/config.yaml")
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

func addConfigFlags(cmd *cobra.Command, config *Config) {
	// read flags
	// Set up global configuration path.
	globalConfig := ReadGlobalConfig()

	// threads
	cmd.PersistentFlags().IntVarP(&config.Threads, "threads", "t", globalConfig.GetInt("default_thread_count"), "Number of threads to run in parallel")

	// default zeek dir
	cmd.PersistentFlags().StringVarP(&config.ZeekLogDir, "logdir", "i",
		globalConfig.GetString("zeek_log_dir"),
		"Zeek log directory",
	)

	cmd.PersistentFlags().BoolVarP(&config.Verbose, "verbose", "v", false, "verbose output")

	cmd.PersistentFlags().BoolVarP(&config.Concat, "concat", "c",
		globalConfig.GetBool("concat_by_default"),
		"concat all output to one file, rather than files for each date.",
	)

	// time range to parse
	cmd.PersistentFlags().StringVarP(
		&config.RawTimeRange, "timerange", "r",
		fmt.Sprintf( // write range of last 24 hours
			"%s-%s",
			time.Now().AddDate(0, 0, -1).Format(TimeFormatShort), // yesterday at this time
			time.Now().Format(TimeFormatShort)),                  // right now
		"time-range (local time). unspecified: last 24 hours. Format: YYYY/MM/DD:HH-YYYY/MM/DD:HH",
	)

	// default path for log storage is ./output-DATE
	// uses this if no path specified.
	defaultPath, e := filepath.Abs("./output-" + time.Now().Format(TimeFormatLongNum))
	if e != nil {
		panic("fatal error: could not resolve relative path")
	}

	cmd.PersistentFlags().StringVarP(&config.OutputDir, "outdir", "o",
		defaultPath,
		"filtered logs output directory",
	)
}
