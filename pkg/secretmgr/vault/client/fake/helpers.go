package fake

import (
	"testing"

	"github.com/jenkins-x/jx-remote/pkg/fakes/fakejxfactory"
	"github.com/jenkins-x/jx-remote/pkg/secretmgr/vault/client"
	"github.com/jenkins-x/jx/pkg/jxfactory"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// NewVaultClientWithFakeKubernetes creates a new vault client using fake k8s
func NewVaultClientWithFakeKubernetes(t *testing.T) (*client.Factory, jxfactory.Factory) {
	ns := "jx"
	k8sObjects := []runtime.Object{
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      client.SecretUnsealKeys,
				Namespace: ns,
				Labels:    map[string]string{},
			},
			Data: map[string][]byte{
				client.KeyUnsealKeys: []byte("dummytoken"),
			},
		},
		&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      client.SecretVaultTLS,
				Namespace: ns,
				Labels:    map[string]string{},
			},
			Data: map[string][]byte{
				client.KeyVaultTLS: []byte("-- dummy cert file --"),
			},
		},
	}
	jxf := fakejxfactory.NewFakeFactoryWithObjects(k8sObjects, nil, ns)
	f, err := client.NewFactoryFromJX(jxf)
	require.NoError(t, err, "could not create vault factory")

	f.DisableCert = true
	return f, jxf
}
