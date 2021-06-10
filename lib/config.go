package lib

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
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

// The High-Level Config
type Config struct {
	DataSources []DataSource `yaml:"data_sources"` // data_sources
}

// Read the YAML config file from the specified path by string input,
// and then populate a struct based on present fields. Returns
// the struct parsed and if there was an error in parsing.
func ParseConfig(filepath string) (configData Config, err error) {
	// Try to read file from config.
	configBuffer, err := ioutil.ReadFile(filepath)

	// Parse the YAML file, and check for generated errors.
	err = yaml.Unmarshal(configBuffer, &configData)
	return configData, err
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

// TODO
func GenRuntimeConfig(globalConfig *viper.Viper, cmd *cobra.Command) {

}
