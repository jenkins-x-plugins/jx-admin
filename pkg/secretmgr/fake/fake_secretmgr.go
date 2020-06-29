package fake

import "github.com/jenkins-x/jx-remote/pkg/secretmgr"

// FakeSecretManager a fake implementation for testing
type FakeSecretManager struct {
	SecretsYAML string
}

// NewFakeSecretManager creates a fake secret manager
func NewFakeSecretManager() secretmgr.SecretManager {
	return &FakeSecretManager{}
}

// UpsertSecrets upserts the secrets
func (f *FakeSecretManager) UpsertSecrets(callback secretmgr.SecretCallback, defaultYaml string) error {
	if f.SecretsYAML == "" {
		f.SecretsYAML = defaultYaml
	}
	answer, err := callback(f.SecretsYAML)
	if err != nil {
		return err
	}
	f.SecretsYAML = answer
	return nil
}

func (f *FakeSecretManager) Kind() string {
	return secretmgr.KindFake
}

func (f *FakeSecretManager) String() string {
	return f.Kind()
}
