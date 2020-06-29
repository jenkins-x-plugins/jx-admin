package client

import (
	"fmt"

	vaultapi "github.com/hashicorp/vault/api"
	"github.com/pkg/errors"
)

// VaultClient a client for vault
type VaultClient struct {
	client *vaultapi.Client
}

// NewVaultClient creates a new client from the factory
func NewVaultClient(f *Factory) (Client, error) {
	client, err := f.NewClient()
	if err != nil {
		return nil, err
	}
	return &VaultClient{client}, nil
}

// Read reads a tree of data from a path
func (v *VaultClient) Read(name string) (map[string]interface{}, error) {
	client := v.client
	path := secretMetadataPath(name)

	secret, err := client.Logical().List(path)
	if err != nil {
		return nil, errors.Wrapf(err, "listing path %q from vault at %s", path, client.Address())
	}
	// lets handle values at this path
	answer, err := v.readValues(name)
	if err != nil {
		return answer, err
	}
	if answer == nil {
		answer = map[string]interface{}{}
	}

	// now lets load any child folders
	if secret != nil && secret.Data != nil {
		keys := secret.Data["keys"]
		if keys != nil {
			for _, s := range keys.([]interface{}) {
				key, ok := s.(string)
				if ok && key != "" {
					values, err := v.readValues(name + "/" + key)
					if err != nil {
						return answer, err
					}
					answer[key] = values
				}
			}
		}
	}
	return answer, nil
}

func (v *VaultClient) readValues(name string) (map[string]interface{}, error) {
	client := v.client
	path := secretPath(name)
	secret, err := client.Logical().Read(path)
	if err != nil {
		return nil, errors.Wrapf(err, "reading path %q from vault at %s", path, client.Address())
	}

	if secret == nil {
		return nil, fmt.Errorf("no path %q not found in vault at %s", path, client.Address())
	}

	if secret.Data != nil {
		value := secret.Data["data"]
		data, ok := value.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("invalid data type for path %q at %s", path, client.Address())
		}
		return data, nil
	}
	return nil, fmt.Errorf("no data found on path %q", path)
}

// Write writes a tree of data to vault
func (v *VaultClient) Write(name string, values map[string]interface{}) error {
	client := v.client
	path := secretPath(name)

	simpleValues := map[string]interface{}{}

	for k, value := range values {
		m, ok := value.(map[string]interface{})
		if ok && m != nil {
			err := v.Write(name+"/"+k, m)
			if err != nil {
				return err
			}
		} else {
			simpleValues[k] = value
		}
	}
	if len(simpleValues) > 0 {
		payload := map[string]interface{}{
			"data": simpleValues,
		}
		_, err := client.Logical().Write(path, payload)
		if err != nil {
			return errors.Wrapf(err, "writing path %s to vault at %s", path, client.Address())
		}
	}
	return nil
}

// String returns a textual representation
func (v *VaultClient) String() string {
	return fmt.Sprintf("vault at %s", v.client.Address())
}

// secretPath generates a secret path from the secret path for storing in vault
// this just makes sure it gets stored under /secret
func secretPath(path string) string {
	return "secret/data/" + path
}

// secretMetaPath generates the secret metadata path form the secret path provided
func secretMetadataPath(path string) string {
	return "secret/metadata/" + path
}
