package lib

import "io/ioutil"
import "gopkg.in/yaml.v2"

type DataSource struct {
	Name string // name
	Threads int // threads
	ManualPath string `yaml:"manual_path"` // manual_path
}

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