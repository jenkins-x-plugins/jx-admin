package proxy

import (
	"github.com/jenkins-x/jx-remote/pkg/secretmgr"
)

// ProxySecretManager stores the updated secret in the secret storage
type ProxySecretManager struct {
	First  secretmgr.SecretManager
	Second secretmgr.SecretManager
}

// NewProxySecretManager uses a Kubernetes Secret to manage secrets
func NewProxySecretManager(first, second secretmgr.SecretManager) secretmgr.SecretManager {
	return &ProxySecretManager{First: first, Second: second}
}

// UpsertSecrets upserts the secrets
func (f *ProxySecretManager) UpsertSecrets(callback secretmgr.SecretCallback, defaultYaml string) error {

	updatedYaml := ""

	proxyCallback := func(secretYaml string) (string, error) {
		y, err := callback(secretYaml)
		if err != nil {
			return y, err
		}
		updatedYaml = y
		return y, nil
	}

	err := f.First.UpsertSecrets(proxyCallback, defaultYaml)
	if err != nil {
		return err
	}

	populateCallback := func(secretYaml string) (string, error) {
		return updatedYaml, nil
	}
	return f.Second.UpsertSecrets(populateCallback, defaultYaml)
}

func (f *ProxySecretManager) Kind() string {
	return f.First.Kind()
}

func (f *ProxySecretManager) String() string {
	return f.First.String()
}
