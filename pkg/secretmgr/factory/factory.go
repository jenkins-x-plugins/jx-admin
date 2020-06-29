package factory

import (
	"fmt"

	"github.com/jenkins-x/jx-remote/pkg/secretmgr"
	"github.com/jenkins-x/jx-remote/pkg/secretmgr/fake"
	"github.com/jenkins-x/jx-remote/pkg/secretmgr/gsm"
	"github.com/jenkins-x/jx-remote/pkg/secretmgr/local"
	"github.com/jenkins-x/jx-remote/pkg/secretmgr/vault"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/jxfactory"
)

// NewSecretManager creates a secret manager from a kind string
func NewSecretManager(kind string, f jxfactory.Factory, requirements *config.RequirementsConfig) (secretmgr.SecretManager, error) {
	if f == nil {
		f = jxfactory.NewFactory()
	}
	switch kind {
	case secretmgr.KindGoogleSecretManager:
		return gsm.NewGoogleSecretManager(requirements)
	case secretmgr.KindLocal:
		return local.NewLocalSecretManager(f, requirements.Cluster.Namespace)
	case secretmgr.KindFake:
		return fake.NewFakeSecretManager(), nil
	case secretmgr.KindVault:
		return vault.NewVaultSecretManagerFromJXFactory(f)
	default:
		return nil, fmt.Errorf("unknown secret manager kind: %s", kind)
	}
}
