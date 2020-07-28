package gitconfig

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"gopkg.in/yaml.v3"
)

const defaultHostname = "github.com"

type ConfigEntry struct {
	User  string
	Token string `yaml:"oauth_token"`
}

func parseOrSetupConfigFile(fn string) (*ConfigEntry, error) {
	entry, err := parseConfigFile(fn)
	if err != nil {
		return setupConfigFile(fn)
	}
	return entry, err
}

func parseConfigFile(fn string) (*ConfigEntry, error) {
	f, err := os.Open(fn)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return parseConfig(f)
}

// ParseDefaultConfig reads the configuration file
func ParseDefaultConfig() (*ConfigEntry, error) {
	return parseConfigFile(configFile())
}

func parseConfig(r io.Reader) (*ConfigEntry, error) {
	data, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}
	var config yaml.Node
	err = yaml.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}
	if len(config.Content) < 1 {
		return nil, fmt.Errorf("malformed config")
	}
	for i := 0; i < len(config.Content[0].Content)-1; i += 2 {
		if config.Content[0].Content[i].Value == defaultHostname {
			var entries []ConfigEntry
			err = config.Content[0].Content[i+1].Decode(&entries)
			if err != nil {
				return nil, err
			}
			return &entries[0], nil
		}
	}
	return nil, fmt.Errorf("could not find config entry for %q", defaultHostname)
}
