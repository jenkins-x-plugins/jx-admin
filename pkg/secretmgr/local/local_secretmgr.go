package local

import (
	"fmt"

	"github.com/jenkins-x/jx-remote/pkg/secretmgr"
	"github.com/jenkins-x/jx/pkg/jxfactory"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// LocalSecretManager uses a Kubernetes Secret
type LocalSecretManager struct {
	KubeClient kubernetes.Interface
	Namespace  string
}

// NewLocalSecretManager uses a Kubernetes Secret to manage secrets
func NewLocalSecretManager(f jxfactory.Factory, namespace string) (secretmgr.SecretManager, error) {
	kubeClient, ns, err := f.CreateKubeClient()
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Kube Client")
	}
	if namespace == "" {
		namespace = ns
	}

	// lets verify the namespace is created if it doesn't exist
	err = kube.EnsureDevNamespaceCreatedWithoutEnvironment(kubeClient, namespace)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to ensure dev namespace setup %s", namespace)
	}
	return &LocalSecretManager{KubeClient: kubeClient, Namespace: namespace}, nil
}

// UpsertSecrets upserts the secrets
func (f *LocalSecretManager) UpsertSecrets(callback secretmgr.SecretCallback, defaultYaml string) error {
	secret, err := f.loadSecret()
	if err != nil {
		return err
	}

	secretYaml := f.getSecretYaml(secret)
	if secretYaml == "" {
		secretYaml = defaultYaml
	}

	updatedYaml, err := callback(secretYaml)
	if err != nil {
		return err
	}
	if updatedYaml != secretYaml {
		return f.updateSecretYaml(updatedYaml)
	}
	return nil
}

func (f *LocalSecretManager) Kind() string {
	return secretmgr.KindLocal
}

func (f *LocalSecretManager) String() string {
	return fmt.Sprintf("%s in namespace %s with Secret %s", f.Kind(), f.Namespace, secretmgr.LocalSecret)
}

func (f *LocalSecretManager) loadSecret() (*corev1.Secret, error) {
	ns := f.Namespace
	name := secretmgr.LocalSecret
	secret, err := f.KubeClient.CoreV1().Secrets(ns).Get(name, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return secret, errors.Wrapf(err, "failed to find Secret %s in namespace %s", name, ns)
		}
		// lets create a default secret
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: name,
				Labels: map[string]string{
					"app": "helmboot",
				},
				Annotations: map[string]string{},
			},
		}
	}
	return secret, nil
}

func (f *LocalSecretManager) getSecretYaml(secret *corev1.Secret) string {
	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}
	data := secret.Data[secretmgr.LocalSecretKey]
	if data == nil {
		return ""
	}
	return string(data)
}

func (f *LocalSecretManager) updateSecretYaml(newYaml string) error {
	secret, err := f.loadSecret()
	if err != nil {
		return err
	}
	if secret.Data == nil {
		secret.Data = map[string][]byte{}
	}
	secret.Data[secretmgr.LocalSecretKey] = []byte(newYaml)

	ns := f.Namespace
	name := secretmgr.LocalSecret
	secretInterface := f.KubeClient.CoreV1().Secrets(ns)
	if secret.ObjectMeta.ResourceVersion == "" {
		// lets create the secret
		_, err = secretInterface.Create(secret)
		if err != nil {
			return errors.Wrapf(err, "failed to create Secret %s in namespace %s", name, ns)
		}
	} else {
		_, err = secretInterface.Update(secret)
		if err != nil {
			return errors.Wrapf(err, "failed to update Secret %s in namespace %s", name, ns)
		}
	}
	return nil
}
