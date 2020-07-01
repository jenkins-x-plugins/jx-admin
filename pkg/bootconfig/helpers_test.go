package bootconfig_test

import (
	"path/filepath"
	"testing"

	"github.com/jenkins-x/jx-admin/pkg/bootconfig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBootFind(t *testing.T) {
	dir := filepath.Join("test_data", "config-root", "namespaces")
	config, fileName, err := bootconfig.LoadBoot(dir, true)
	require.NoError(t, err)
	require.NotEmpty(t, fileName, "no fileName returned")
	require.NotNil(t, config, "config not returned")

	assert.Equal(t, "jenkins-x-labs-bot", config.Spec.PipelineBotUser, "spec.PipelineBotUser")
	assert.Equal(t, "external-secrets", config.Spec.SecretManager, "spec.PipelineBotUser")
}
