package fakejxfactory

import (
	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	v1fake "github.com/jenkins-x/jx/pkg/client/clientset/versioned/fake"
	"github.com/jenkins-x/jx/pkg/jxfactory"
	"github.com/jenkins-x/jx/pkg/kube"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"

	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	tektonfake "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/fake"
	corev1 "k8s.io/api/core/v1"
)

// FakeFactory represents a fake factory
type FakeFactory struct {
	KubeClient   kubernetes.Interface
	JXClient     versioned.Interface
	TektonClient tektonclient.Interface
	Namespace    string
}

// NewFakeFactory returns a fake factory for testing
func NewFakeFactory() jxfactory.Factory {
	return NewFakeFactoryWithObjects(nil, nil, "jx")
}

// NewFakeFactory returns a fake factory for testing with the given initial objects
func NewFakeFactoryWithObjects(kubeObjects []runtime.Object, jxObjects []runtime.Object, namespace string) jxfactory.Factory {
	if len(kubeObjects) == 0 {
		kubeObjects = append(kubeObjects, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
				Labels: map[string]string{
					"tag":  "",
					"team": "jx",
					"env":  "dev",
				},
			},
		})
	}
	return &FakeFactory{
		KubeClient:   fake.NewSimpleClientset(kubeObjects...),
		JXClient:     v1fake.NewSimpleClientset(jxObjects...),
		TektonClient: tektonfake.NewSimpleClientset(),
		Namespace:    namespace,
	}
}

func (f *FakeFactory) WithBearerToken(token string) jxfactory.Factory {
	return f
}

func (f *FakeFactory) ImpersonateUser(user string) jxfactory.Factory {
	return f
}

func (f *FakeFactory) CreateKubeClient() (kubernetes.Interface, string, error) {
	return f.KubeClient, f.Namespace, nil
}

func (f *FakeFactory) CreateKubeConfig() (*rest.Config, error) {
	return &rest.Config{}, nil
}

func (f *FakeFactory) CreateJXClient() (versioned.Interface, string, error) {
	return f.JXClient, f.Namespace, nil
}

func (f *FakeFactory) CreateTektonClient() (tektonclient.Interface, string, error) {
	return f.TektonClient, f.Namespace, nil
}

func (f *FakeFactory) KubeConfig() kube.Kuber {
	return f
}

func (f *FakeFactory) LoadConfig() (*api.Config, *clientcmd.PathOptions, error) {
	return &api.Config{
		CurrentContext: "current",
		Contexts: map[string]*api.Context{
			"current": {
				Namespace: f.Namespace,
			},
		},
	}, &clientcmd.PathOptions{}, nil
}

func (f *FakeFactory) UpdateConfig(namespace string, server string, caData string, user string, token string) error {
	panic("implement me")
}
