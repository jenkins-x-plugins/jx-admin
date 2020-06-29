package client

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	vaultapi "github.com/hashicorp/vault/api"
	"github.com/jenkins-x/jx-remote/pkg/clienthelpers"
	"github.com/jenkins-x/jx/pkg/jxfactory"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// SecretUnsealKeys name of the secret for unsealing keys
	SecretUnsealKeys = "vault-unseal-keys"

	// KeyUnsealKeys secret key for the unseal key
	KeyUnsealKeys = "vault-root"

	// SecretVaultTLS secret name for TLS
	SecretVaultTLS = "vault-tls"

	// KeyVaultTLS secret key for TLS cert
	KeyVaultTLS = "ca.crt"

	defaultCertPath = "vault-ca.crt"
)

// Factory is a simple vault client factory which initialises a vault client
type Factory struct {
	CertFile    string
	DisableCert bool
	kubeClient  kubernetes.Interface
	namespace   string
}

// NewFactory creates a new factory
func NewFactory(kubeClient kubernetes.Interface, namespace string) *Factory {
	disableCert := os.Getenv("JX_DISABLE_VAULT_CERT") == "true"
	return &Factory{CertFile: "", DisableCert: disableCert, kubeClient: kubeClient, namespace: namespace}
}

// NewFactoryFromJX creates a new vault factory from a jx client factory
func NewFactoryFromJX(f jxfactory.Factory) (*Factory, error) {
	kubeClient, ns, err := f.CreateKubeClient()
	if err != nil {
		return nil, err
	}
	return NewFactory(kubeClient, ns), nil
}

// NewClient creates a new vault client
func (f *Factory) NewClient() (*vaultapi.Client, error) {
	// lets check if we have a vault token....
	token := os.Getenv(vaultapi.EnvVaultToken)
	if token == "" {
		// lets load the token from kubernetes
		token, err := f.loadVaultToken()
		if err != nil {
			return nil, err
		}
		os.Setenv(vaultapi.EnvVaultToken, token)
	}

	// if in cluster use service as the address
	address := os.Getenv(vaultapi.EnvVaultAddress)
	if address == "" && clienthelpers.IsInCluster() {
		os.Setenv(vaultapi.EnvVaultAddress, "https://vault:8200")
	}

	if !f.DisableCert {
		certFile := os.Getenv(vaultapi.EnvVaultCACert)
		if certFile == "" {
			certFile = f.CertFile
			if certFile == "" {
				tmpDir, err := ioutil.TempDir("", "")
				if err != nil {
					return nil, errors.Wrap(err, "failed to create a temp dir for the vault cert")
				}
				certFile = filepath.Join(tmpDir, defaultCertPath)
			}
			os.Setenv(vaultapi.EnvVaultCACert, certFile)
		}
		err := f.ensureCertFileExists(certFile)
		if err != nil {
			return nil, err
		}
	}

	config := vaultapi.DefaultConfig()
	if config == nil {
		return nil, fmt.Errorf("no default config created")
	}
	return vaultapi.NewClient(config)
}

func (f *Factory) loadVaultToken() (string, error) {
	return f.getSecretKey(SecretUnsealKeys, KeyUnsealKeys)
}

func (f *Factory) getSecretKey(name string, key string) (string, error) {
	namespace := f.namespace
	secret, err := f.kubeClient.CoreV1().Secrets(namespace).Get(name, metav1.GetOptions{})
	if err != nil {
		return "", errors.Wrapf(err, "failed to find Secret %s in namespace %s", name, namespace)
	}
	var answer []byte
	if secret != nil && secret.Data != nil {
		answer = secret.Data[key]
	}
	if len(answer) == 0 {
		return "", errors.Errorf("no data for key %s in Secret %s in namespace %s", key, name, namespace)
	}
	return string(answer), nil
}

func (f *Factory) ensureCertFileExists(file string) error {
	exists, err := util.FileExists(file)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	cert, err := f.getSecretKey(SecretVaultTLS, KeyVaultTLS)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(file, []byte(cert), util.DefaultFileWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to save Vault Cert file to %s", f.CertFile)
	}
	log.Logger().Infof("saved vault cert file to %s", util.ColorInfo(file))
	return nil
}
