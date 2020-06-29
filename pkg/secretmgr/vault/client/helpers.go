package client

import (
	"github.com/jenkins-x/jx-remote/pkg/secretmgr"
	"github.com/pkg/errors"
)

// ReadYaml reads the secrets yaml from the vault client
func ReadYaml(client Client, path string) (string, error) {
	m, err := client.Read(path)
	if err != nil {
		return "", err
	}
	return secretmgr.ToSecretsYAML(m)
}

// WriteYAML writes the vault YAML to the given path
func WriteYAML(client Client, path string, yaml string) error {
	values, err := secretmgr.UnmarshalSecretsYAML(yaml)
	if err != nil {
		return err
	}
	err = client.Write(path, values)
	if err != nil {
		return errors.Wrapf(err, "failed to write to %s for path %s", client.String(), path)
	}
	return nil
}
