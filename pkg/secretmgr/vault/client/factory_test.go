package client_test

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/jenkins-x/jx-remote/pkg/secretmgr/vault/client"
	"github.com/jenkins-x/jx-remote/pkg/secretmgr/vault/client/fake"
	"github.com/jenkins-x/jx/pkg/jxfactory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient(t *testing.T) {
	// lets only test if we have a local vault
	if os.Getenv("TEST_VAULT") != "true" {
		t.SkipNow()
		return
	}

	jxf := jxfactory.NewFactory()
	f, err := client.NewFactoryFromJX(jxf)
	require.NoError(t, err, "could not create vault factory")

	AssertVaultClientOperations(t, f)
}

func TestClientWithFakeServer(t *testing.T) {
	f, _ := fake.NewVaultClientWithFakeKubernetes(t)

	// lets create a fake test vault server...
	server := fake.NewFakeVaultServer(t)

	defer server.Close()
	AssertVaultClientOperations(t, f)
}

// AssertVaultClientOperations performs tests on the vault client to check it works
func AssertVaultClientOperations(t *testing.T, f *client.Factory) {
	// lets create a temp file
	tempDir, err := ioutil.TempDir("", "vault-cert-")
	require.NoError(t, err, "failed to create a temporary file")
	f.CertFile = filepath.Join(tempDir, "vault-ca.crt")
	defer os.RemoveAll(tempDir)

	vaultClient, err := client.NewVaultClient(f)
	require.NoError(t, err, "could not create vault client")

	t.Logf("Created Vault client")

	path := "thingy"
	expectedData := map[string]interface{}{
		"hmacToken": "TODO",
		"another":   "thing",
		"adminUser": map[string]interface{}{
			"username": "admin",
			"password": "dummypwd",
		},
		"pipelineUser": map[string]interface{}{
			"username": "somegithyser",
			"token":    "sometoken",
		},
	}
	err = vaultClient.Write(path, expectedData)
	require.NoError(t, err, "failed to write data %v to vault", expectedData)

	actual, err := vaultClient.Read(path)
	require.NoError(t, err, "could not read from vault")
	require.NotNil(t, actual, "no data found in vault")

	for k, v := range actual {
		t.Logf("  %s ->  %+v", k, v)
	}
	t.Logf("Finished reading Vault for path: %s\n", path)

	assert.Equal(t, expectedData, actual, "data read from vault")
}
