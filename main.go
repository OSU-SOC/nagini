package main

import (
	fmt "fmt"

	cmd "github.com/OSU-SOC/nagini/cmd"
	viper "github.com/spf13/viper"
)

func main() {
	// Set up global configuration path.
	globalConfig := viper.New()
	globalConfig.SetConfigName("config")
	globalConfig.SetConfigType("yaml")
	// Config paths in order of priority.
	globalConfig.AddConfigPath("/etc/nagini/")
	globalConfig.AddConfigPath("$HOME/.config/nagini/")

	// Try ingesting config from one of the config paths.
	if err := globalConfig.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			panic(fmt.Errorf("No config file present. Please add one to /etc/nagini or ~/.config/nagini"))
		} else {
			// Config file was found but another error was produced
			panic(fmt.Errorf("Unexpected Error: %s", err))
		}
	}
	cmd.Execute()
}
