package fakeclientsfactory

import (
	"github.com/heptio/sonobuoy/pkg/client"
	gojenkins "github.com/jenkins-x/golang-jenkins"
	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/cmd/clients"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/helm"
	"github.com/jenkins-x/jx/pkg/io/secrets"
	"github.com/jenkins-x/jx/pkg/jxfactory"
	"github.com/jenkins-x/jx/pkg/kustomize"
	"github.com/jenkins-x/jx/pkg/util"
	"k8s.io/client-go/kubernetes"

	"k8s.io/client-go/dynamic"

	"io"

	"github.com/jenkins-x/jx/pkg/vault"
	certmngclient "github.com/jetstack/cert-manager/pkg/client/clientset/versioned"

	"github.com/jenkins-x/jx/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/pkg/table"
	"k8s.io/client-go/rest"

	vaultoperatorclient "github.com/banzaicloud/bank-vaults/operator/pkg/client/clientset/versioned"
	kserve "github.com/knative/serving/pkg/client/clientset/versioned"
	tektonclient "github.com/tektoncd/pipeline/pkg/client/clientset/versioned"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"

	// this is so that we load the auth plugins so we can connect to, say, GCP

	_ "k8s.io/client-go/plugin/pkg/client/auth"

	prowjobclient "k8s.io/test-infra/prow/client/clientset/versioned"
)

// fakeClientsFactory a fake factory
type fakeClientsFactory struct {
	jxf            jxfactory.Factory
	defaultFactory clients.Factory
}

// NewFakeFactory returns a fake factory for testing
func NewFakeFactory(jxf jxfactory.Factory, defaultFactory clients.Factory) clients.Factory {
	if defaultFactory == nil {
		defaultFactory = clients.NewFactory()
	}
	return &fakeClientsFactory{jxf, defaultFactory}
}

func (f *fakeClientsFactory) WithBearerToken(token string) clients.Factory {
	f.jxf.WithBearerToken(token)
	return f
}

func (f *fakeClientsFactory) ImpersonateUser(user string) clients.Factory {
	f.jxf.ImpersonateUser(user)
	return f
}

func (f *fakeClientsFactory) CreateAuthConfigService(fileName string, namespace string, serverKind string, serviceKind string) (auth.ConfigService, error) {
	return f.defaultFactory.CreateAuthConfigService(fileName, namespace, serverKind, serviceKind)
}

func (f *fakeClientsFactory) CreateGitAuthConfigService(namespace string, serviceKind string) (auth.ConfigService, error) {
	return f.defaultFactory.CreateGitAuthConfigService(namespace, serviceKind)
}

func (f *fakeClientsFactory) CreateLocalGitAuthConfigService() (auth.ConfigService, error) {
	return f.defaultFactory.CreateLocalGitAuthConfigService()
}

func (f *fakeClientsFactory) CreateJenkinsAuthConfigService(namespace string, jenkinsService string) (auth.ConfigService, error) {
	return f.defaultFactory.CreateJenkinsAuthConfigService(namespace, jenkinsService)
}

func (f *fakeClientsFactory) CreateChartmuseumAuthConfigService(namespace string, serviceKind string) (auth.ConfigService, error) {
	return f.defaultFactory.CreateChartmuseumAuthConfigService(namespace, serviceKind)
}

func (f *fakeClientsFactory) CreateIssueTrackerAuthConfigService(namespace string, serviceKind string) (auth.ConfigService, error) {
	return f.defaultFactory.CreateIssueTrackerAuthConfigService(namespace, serviceKind)
}

func (f *fakeClientsFactory) CreateChatAuthConfigService(namespace string, serviceKind string) (auth.ConfigService, error) {
	return f.defaultFactory.CreateChatAuthConfigService(namespace, serviceKind)
}

func (f *fakeClientsFactory) CreateAddonAuthConfigService(namespace string, serviceKind string) (auth.ConfigService, error) {
	return f.defaultFactory.CreateAddonAuthConfigService(namespace, serviceKind)
}

func (f *fakeClientsFactory) CreateJenkinsClient(kubeClient kubernetes.Interface, ns string, handles util.IOFileHandles) (gojenkins.JenkinsClient, error) {
	return f.defaultFactory.CreateJenkinsClient(kubeClient, ns, handles)
}

func (f *fakeClientsFactory) CreateCustomJenkinsClient(kubeClient kubernetes.Interface, ns string, jenkinsServiceName string, handles util.IOFileHandles) (gojenkins.JenkinsClient, error) {
	return f.defaultFactory.CreateCustomJenkinsClient(kubeClient, ns, jenkinsServiceName, handles)
}

func (f *fakeClientsFactory) CreateGitProvider(gitURL string, message string, authConfigSvc auth.ConfigService, gitKind string, ghOwner string, batchMode bool, gitter gits.Gitter, handles util.IOFileHandles) (gits.GitProvider, error) {
	return f.defaultFactory.CreateGitProvider(gitURL, message, authConfigSvc, gitKind, ghOwner, batchMode, gitter, handles)
}

func (f *fakeClientsFactory) CreateComplianceClient() (*client.SonobuoyClient, error) {
	return f.defaultFactory.CreateComplianceClient()
}

func (f *fakeClientsFactory) CreateSystemVaultClient(namespace string) (vault.Client, error) {
	return f.defaultFactory.CreateSystemVaultClient(namespace)
}

func (f *fakeClientsFactory) CreateVaultClient(name string, namespace string) (vault.Client, error) {
	return f.defaultFactory.CreateVaultClient(name, namespace)
}

func (f *fakeClientsFactory) CreateHelm(verbose bool, helmBinary string, noTiller bool, helmTemplate bool) helm.Helmer {
	return f.defaultFactory.CreateHelm(verbose, helmBinary, noTiller, helmTemplate)
}

func (f *fakeClientsFactory) CreateKubeClient() (kubernetes.Interface, string, error) {
	return f.jxf.CreateKubeClient()
}

func (f *fakeClientsFactory) CreateKubeConfig() (*rest.Config, error) {
	return f.jxf.CreateKubeConfig()
}

func (f *fakeClientsFactory) CreateJXClient() (versioned.Interface, string, error) {
	return f.jxf.CreateJXClient()
}

func (f *fakeClientsFactory) CreateApiExtensionsClient() (apiextensionsclientset.Interface, error) {
	return f.defaultFactory.CreateApiExtensionsClient()
}

func (f *fakeClientsFactory) CreateDynamicClient() (dynamic.Interface, string, error) {
	return f.defaultFactory.CreateDynamicClient()
}

func (f *fakeClientsFactory) CreateMetricsClient() (metricsclient.Interface, error) {
	return f.defaultFactory.CreateMetricsClient()
}

func (f *fakeClientsFactory) CreateTektonClient() (tektonclient.Interface, string, error) {
	return f.jxf.CreateTektonClient()
}

func (f *fakeClientsFactory) CreateProwJobClient() (prowjobclient.Interface, string, error) {
	return f.defaultFactory.CreateProwJobClient()
}

func (f *fakeClientsFactory) CreateKnativeServeClient() (kserve.Interface, string, error) {
	return f.defaultFactory.CreateKnativeServeClient()
}

func (f *fakeClientsFactory) CreateVaultOperatorClient() (vaultoperatorclient.Interface, error) {
	return f.defaultFactory.CreateVaultOperatorClient()
}

func (f *fakeClientsFactory) CreateCertManagerClient() (certmngclient.Interface, error) {
	return f.defaultFactory.CreateCertManagerClient()
}

func (f *fakeClientsFactory) CreateTable(out io.Writer) table.Table {
	return f.defaultFactory.CreateTable(out)

}

func (f *fakeClientsFactory) GetJenkinsURL(kubeClient kubernetes.Interface, ns string) (string, error) {
	return f.defaultFactory.GetJenkinsURL(kubeClient, ns)
}

func (f *fakeClientsFactory) GetCustomJenkinsURL(kubeClient kubernetes.Interface, ns string, jenkinsServiceName string) (string, error) {
	return f.defaultFactory.GetCustomJenkinsURL(kubeClient, ns, jenkinsServiceName)
}

func (f *fakeClientsFactory) SetBatch(batch bool) {
	f.defaultFactory.SetBatch(batch)

}

func (f *fakeClientsFactory) SetOffline(offline bool) {
	f.defaultFactory.SetOffline(offline)
}

func (f *fakeClientsFactory) IsInCDPipeline() bool {
	return f.defaultFactory.IsInCDPipeline()
}

func (f *fakeClientsFactory) SecretsLocation() secrets.SecretsLocationKind {
	return f.defaultFactory.SecretsLocation()
}

func (f *fakeClientsFactory) SetSecretsLocation(location secrets.SecretsLocationKind, persist bool) error {
	return f.defaultFactory.SetSecretsLocation(location, persist)
}

func (f *fakeClientsFactory) ResetSecretsLocation() {
	f.defaultFactory.ResetSecretsLocation()
}

func (f *fakeClientsFactory) CreateKustomizer() kustomize.Kustomizer {
	return nil
}
