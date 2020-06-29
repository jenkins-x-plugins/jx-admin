package factory_test

import (
	"os"
	"testing"

	"github.com/jenkins-x/jx-remote/pkg/fakes/fakejxfactory"
	"github.com/jenkins-x/jx-remote/pkg/secretmgr"
	"github.com/jenkins-x/jx-remote/pkg/secretmgr/factory"
	"github.com/jenkins-x/jx-remote/pkg/secretmgr/fake"
	vaultfake "github.com/jenkins-x/jx-remote/pkg/secretmgr/vault/client/fake"
	"github.com/jenkins-x/jx-remote/pkg/testhelpers"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/jxfactory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	modifiedYaml = `secrets:
  adminUser:
    username: admin
    password: dummypwd 
  hmacToken:  TODO
  pipelineUser:
    username: someuser 
    token: dummmytoken 
    email: me@foo.com
`
)

func TestFakeSecretManager(t *testing.T) {
	f := fakejxfactory.NewFakeFactory()
	sm := AssertSecretsManager(t, secretmgr.KindFake, f)

	// lets assume its a fake one
	fakeSM, ok := sm.(*fake.FakeSecretManager)
	require.True(t, ok, "SecretManager should be Fake but was %#v", sm)
	assert.Equal(t, modifiedYaml, fakeSM.SecretsYAML, "FakeSecretManager should contain the correct YAML")
}

func TestVaultSecretManagerWithFakeVaultServer(t *testing.T) {
	_, jxf := vaultfake.NewVaultClientWithFakeKubernetes(t)

	// lets create a fake test vault server...
	server := vaultfake.NewFakeVaultServer(t)
	defer server.Close()

	// disable vault cert for testing
	os.Setenv("JX_DISABLE_VAULT_CERT", "true")
	defer os.Setenv("JX_DISABLE_VAULT_CERT", "false")

	AssertSecretsManager(t, secretmgr.KindVault, jxf)
}

func TestLocalSecretManager(t *testing.T) {
	f := fakejxfactory.NewFakeFactory()
	AssertSecretsManager(t, secretmgr.KindLocal, f)

	// lets assume its a fake one
	kubeClient, ns, err := f.CreateKubeClient()
	require.NoError(t, err, "faked to create KubeClient")
	secret, err := kubeClient.CoreV1().Secrets(ns).Get(secretmgr.LocalSecret, metav1.GetOptions{})
	require.NoError(t, err, "failed to get Secret %s in namespace %s", secretmgr.LocalSecret, ns)

	secretYaml := string(secret.Data[secretmgr.LocalSecretKey])
	testhelpers.AssertYamlEqual(t, modifiedYaml, secretYaml, "should have got the YAML from the vault secret manager")
}

func AssertSecretsManager(t *testing.T, kind string, f jxfactory.Factory) secretmgr.SecretManager {
	requirements := config.NewRequirementsConfig()
	sm, err := factory.NewSecretManager(kind, f, requirements)
	require.NoError(t, err, "failed to create a SecretManager of kind %s", kind)
	require.NotNil(t, sm, "SecretManager of kind %s", kind)

	err = sm.UpsertSecrets(dummyCallback, secretmgr.DefaultSecretsYaml)
	require.NoError(t, err, "failed to modify secrets for SecretManager of kind %s", kind)

	actualYaml := ""
	testCb := func(secretsYaml string) (string, error) {
		actualYaml = secretsYaml
		return secretsYaml, nil
	}

	err = sm.UpsertSecrets(testCb, secretmgr.DefaultSecretsYaml)
	require.NoError(t, err, "failed to get the secrets from the SecretManager of kind %s", kind)

	testhelpers.AssertYamlEqual(t, modifiedYaml, actualYaml, "should have got the YAML from the secret manager kind %s", kind)
	return sm
}

func dummyCallback(secretsYaml string) (string, error) {
	return modifiedYaml, nil
}
