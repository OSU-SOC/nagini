package lib

import (
	"io/ioutil"

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
