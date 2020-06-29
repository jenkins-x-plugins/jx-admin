package gitconfig

import (
	"path"

	"github.com/mitchellh/go-homedir"
)

// Context represents the interface for querying information about the current environment
type Context interface {
	AuthToken() (string, error)
	SetAuthToken(string)
	AuthLogin() (string, error)
}

// New initializes a Context that reads from the filesystem
func New() Context {
	return &fsContext{}
}

// A Context implementation that queries the filesystem
type fsContext struct {
	config    *configEntry
	authToken string
}

func ConfigDir() string {
	dir, _ := homedir.Expand("~/.config/jxl")
	return dir
}

func configFile() string {
	return path.Join(ConfigDir(), "config.yml")
}

func (c *fsContext) getConfig() (*configEntry, error) {
	if c.config == nil {
		entry, err := parseOrSetupConfigFile(configFile())
		if err != nil {
			return nil, err
		}
		c.config = entry
		c.authToken = ""
	}
	return c.config, nil
}

func (c *fsContext) AuthToken() (string, error) {
	if c.authToken != "" {
		return c.authToken, nil
	}

	config, err := c.getConfig()
	if err != nil {
		return "", err
	}
	return config.Token, nil
}

func (c *fsContext) SetAuthToken(t string) {
	c.authToken = t
}

func (c *fsContext) AuthLogin() (string, error) {
	config, err := c.getConfig()
	if err != nil {
		return "", err
	}
	return config.User, nil
}
