package vault_test

import (
	"testing"

	"github.com/jenkins-x/jx-remote/pkg/secretmgr"
	"github.com/jenkins-x/jx-remote/pkg/secretmgr/vault"
	"github.com/jenkins-x/jx-remote/pkg/secretmgr/vault/client/fake"
	"github.com/jenkins-x/jx-remote/pkg/testhelpers"
	"github.com/stretchr/testify/require"
)

const (
	initialYaml = `secrets:
  adminUser:
    username: admin
    password: dummypwd 
  hmacToken:  TODO
  pipelineUser:
    username: someuser 
    token: dummmytoken 
    email: me@foo.com
`

	updatedYaml = `secrets:
  adminUser:
    username: admin
    password: newdummypwd 
  hmacToken:  TODO
  pipelineUser:
    username: someuser 
    token: newdummmytoken 
    email: me@foo.com
`
)

func TestVaultSecretManager(t *testing.T) {
	path := "jx"

	client := &fake.FakeClient{}
	sm, err := vault.NewVaultSecretManager(client, path)

	require.NoError(t, err, "failed to create a Vault SecretManager")
	require.NotNil(t, sm, "nil SecretManager")

	// lets populate the yaml
	err = sm.UpsertSecrets(initialiseCallback, secretmgr.DefaultSecretsYaml)
	require.NoError(t, err, "failed to modify secrets for Vault SecretManager")

	actualYaml := ""
	loadCallback := func(secretsYaml string) (string, error) {
		actualYaml = secretsYaml
		return secretsYaml, nil
	}

	err = sm.UpsertSecrets(loadCallback, secretmgr.DefaultSecretsYaml)
	require.NoError(t, err, "failed to get the secrets from the Vault SecretManager")

	testhelpers.AssertYamlEqual(t, initialYaml, actualYaml, "should have got the YAML from the vault secret manager")

	// now lets modify
	err = sm.UpsertSecrets(modifyCallback, secretmgr.DefaultSecretsYaml)
	require.NoError(t, err, "failed to modify the secrets from the Vault SecretManager")

	err = sm.UpsertSecrets(loadCallback, secretmgr.DefaultSecretsYaml)
	require.NoError(t, err, "failed to get the secrets from the Vault SecretManager")

	testhelpers.AssertYamlEqual(t, updatedYaml, actualYaml, "should have modified the YAML in the vault secret manager")

	for n, m := range client.Data {
		if m != nil {
			for k, v := range m {
				t.Logf("got data %s / %v = %v", n, k, v)
			}
		}
	}
}

func initialiseCallback(secretsYaml string) (string, error) {
	return initialYaml, nil
}

func modifyCallback(secretsYaml string) (string, error) {
	return updatedYaml, nil
}
