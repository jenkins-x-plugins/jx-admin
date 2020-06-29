package reqhelpers

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/jenkins-x/jx-apps/pkg/jxapps"
	"github.com/jenkins-x/jx-logging/pkg/log"
	"github.com/jenkins-x/jx-remote/pkg/common"
	v1 "github.com/jenkins-x/jx/v2/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/v2/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx/v2/pkg/cloud"
	"github.com/jenkins-x/jx/v2/pkg/config"
	"github.com/jenkins-x/jx/v2/pkg/gits"
	"github.com/jenkins-x/jx/v2/pkg/jxfactory"
	"github.com/jenkins-x/jx/v2/pkg/kube"
	"github.com/jenkins-x/jx/v2/pkg/kube/naming"
	"github.com/jenkins-x/jx/v2/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
)

// RequirementFlags for the boolean flags we only update if specified on the CLI
type RequirementFlags struct {
	Repository                                                      string
	IngressKind                                                     string
	SecretStorage                                                   string
	AutoUpgrade, EnvironmentGitPublic, GitPublic, EnvironmentRemote bool
	GitOps, Kaniko, Terraform, TLS                                  bool
	Canary, HPA                                                     bool
	VaultRecreateBucket, VaultDisableURLDiscover                    bool
}

// GetDevEnvironmentConfig returns the dev environment for the given requirements or nil
func GetDevEnvironmentConfig(requirements *config.RequirementsConfig) *config.EnvironmentConfig {
	for i, e := range requirements.Environments {
		if e.Key == "dev" {
			return &requirements.Environments[i]
		}
	}
	return nil
}

// GetBootJobCommand returns the boot job command
func GetBootJobCommand(requirements *config.RequirementsConfig, gitURL, gitUser, gitToken, chartName, version, repo, tag, helmBin string, secretsYAML bool) util.Command {
	args := []string{"install", "jx-boot"}

	provider := requirements.Cluster.Provider
	if provider != "" {
		args = append(args, "--set", fmt.Sprintf("jxRequirements.cluster.provider=%s", provider))
	}

	project := requirements.Cluster.ProjectID
	if project != "" {
		args = append(args, "--set", fmt.Sprintf("jxRequirements.cluster.project=%s", project))
	}

	clusterName := requirements.Cluster.ClusterName
	if clusterName != "" {
		args = append(args, "--set", fmt.Sprintf("jxRequirements.cluster.clusterName=%s", clusterName))
	}

	if gitURL != "" {
		args = append(args, "--set", fmt.Sprintf("jxRequirements.bootConfigURL=%s", gitURL))
	}
	if repo != "" {
		args = append(args, "--set", fmt.Sprintf("image.repository=%s", repo))
	}
	if tag != "" {
		args = append(args, "--set", fmt.Sprintf("image.tag=%s", tag))
	}
	if gitUser != "" {
		args = append(args, "--set", fmt.Sprintf("git.username=%s", gitUser))
	}
	if gitToken != "" {
		args = append(args, "--set", fmt.Sprintf("git.token=%s", gitToken))
	}
	args = append(args, "--set", fmt.Sprintf("secrets.yaml=%v", secretsYAML))

	if version != "" {
		args = append(args, "--version", version)
	}
	args = append(args, chartName)

	return util.Command{
		Name: helmBin,
		Args: args,
	}
}

// GetRequirementsFromEnvironment tries to find the development environment then the requirements from it
func GetRequirementsFromEnvironment(kubeClient kubernetes.Interface, jxClient versioned.Interface, namespace string) (*v1.Environment, *config.RequirementsConfig, error) {
	ns, _, err := kube.GetDevNamespace(kubeClient, namespace)
	if err != nil {
		log.Logger().Warnf("could not find the dev namespace from namespace %s due to %s", namespace, err.Error())
		ns = namespace
	}
	devEnv, err := kube.GetDevEnvironment(jxClient, ns)
	if err != nil {
		log.Logger().Warnf("could not find dev Environment in namespace %s", ns)
	}
	if devEnv != nil {
		requirements, err := config.GetRequirementsConfigFromTeamSettings(&devEnv.Spec.TeamSettings)
		if err != nil {
			return devEnv, nil, errors.Wrapf(err, "failed to load requirements from dev environment %s in namespace %s", devEnv.Name, ns)
		}
		if requirements != nil {
			return devEnv, requirements, nil
		}
	}

	// no dev environment found so lets return an empty environment
	if devEnv == nil {
		devEnv = kube.CreateDefaultDevEnvironment(ns)
	}
	if devEnv.Namespace == "" {
		devEnv.Namespace = ns
	}
	requirements := config.NewRequirementsConfig()
	return devEnv, requirements, nil
}

// GetRequirementsFromGit clones the given git repository to get the requirements
func GetRequirementsFromGit(gitURL string) (*config.RequirementsConfig, error) {
	tempDir, err := ioutil.TempDir("", "jx-boot-")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create temp dir")
	}

	log.Logger().Debugf("cloning %s to %s", gitURL, tempDir)

	gitter := gits.NewGitCLI()
	err = gitter.Clone(gitURL, tempDir)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to git clone %s to dir %s", gitURL, tempDir)
	}

	requirements, _, err := config.LoadRequirementsConfig(tempDir, false)
	if err != nil {
		return requirements, errors.Wrapf(err, "failed to requirements YAML file from %s", tempDir)
	}
	return requirements, nil
}

// OverrideRequirements allows CLI overrides
func OverrideRequirements(cmd *cobra.Command, args []string, dir string, customRequirementsFile string, outputRequirements *config.RequirementsConfig, flags *RequirementFlags, environment string) error {
	requirements, fileName, err := config.LoadRequirementsConfig(dir, false)
	if err != nil {
		return err
	}

	if customRequirementsFile != "" {
		exists, err := util.FileExists(customRequirementsFile)
		if err != nil {
			return errors.Wrapf(err, "failed to check if file exists: %s", customRequirementsFile)
		}
		if !exists {
			return fmt.Errorf("custom requirements file %s does not exist", customRequirementsFile)
		}
		requirements, err = config.LoadRequirementsConfigFile(customRequirementsFile, false)
		if err != nil {
			return errors.Wrapf(err, "failed to load: %s", customRequirementsFile)
		}

		UpgradeExistingRequirements(requirements)
	}

	*outputRequirements = *requirements

	// lets re-parse the CLI arguments to re-populate the loaded requirements
	if len(args) == 0 {
		args = os.Args

		// lets trim the actual command which could be `helmboot create` or `jxl boot create` or `jx alpha boot create`
		for i := range args {
			if i == 0 {
				continue
			}
			if i > 3 {
				break
			}
			if args[i] == "create" {
				args = args[i+1:]
				break
			}
		}
	}

	err = cmd.Flags().Parse(args)
	if err != nil {
		return errors.Wrap(err, "failed to reparse arguments")
	}

	err = applyDefaults(cmd, outputRequirements, flags)
	if err != nil {
		return err
	}

	if environment != "" && environment != "dev" {
		// lets make sure we have at least one environment in the requirements
		if len(outputRequirements.Environments) == 0 {
			promoteStrategy := v1.PromotionStrategyTypeManual
			if environment == "staging" {
				promoteStrategy = v1.PromotionStrategyTypeAutomatic
			}
			repository := ""
			clusterName := outputRequirements.Cluster.ClusterName
			if clusterName != "" {
				repository = naming.ToValidName("environment-" + clusterName + "-" + environment)
			}
			outputRequirements.Environments = append(outputRequirements.Environments, config.EnvironmentConfig{
				Key:               "dev",
				Owner:             outputRequirements.Cluster.EnvironmentGitOwner,
				Repository:        repository,
				GitServer:         outputRequirements.Cluster.GitServer,
				GitKind:           outputRequirements.Cluster.GitKind,
				RemoteCluster:     true,
				PromotionStrategy: promoteStrategy,
			})
		}
		outputRequirements.Webhook = config.WebhookTypeLighthouse
	}

	// lets default an ingress service type if there is none
	/* TODO
	if outputRequirements.Ingress.ServiceType == "" {
		outputRequirements.Ingress.ServiceType = "LoadBalancer"
	}
	*/

	if outputRequirements.VersionStream.URL == "" {
		outputRequirements.VersionStream.URL = common.DefaultVersionsURL
	}
	if string(outputRequirements.SecretStorage) == "" {
		outputRequirements.SecretStorage = config.SecretStorageTypeLocal
	}

	err = outputRequirements.SaveConfig(fileName)
	if err != nil {
		return errors.Wrapf(err, "failed to save %s", fileName)
	}

	log.Logger().Infof("saved file: %s", util.ColorInfo(fileName))
	return nil
}

// UpgradeExistingRequirements updates a custom requirements file for helm 3
func UpgradeExistingRequirements(requirements *config.RequirementsConfig) {
	requirements.GitOps = true
	requirements.Helmfile = true
	requirements.Webhook = config.WebhookTypeLighthouse
}

// ValidateApps validates the apps match the requirements
func ValidateApps(dir string, addApps []string, removeApps []string) (*jxapps.AppConfig, string, error) {
	requirements, _, err := config.LoadRequirementsConfig(dir, false)
	if err != nil {
		return nil, "", err
	}
	apps, appsFileName, err := jxapps.LoadAppConfig(dir)

	modified := false
	if requirements.Repository != config.RepositoryTypeNexus {
		if removeApp(apps, "jenkins-x/nexus", appsFileName) {
			modified = true
		}
	}
	if requirements.Repository == config.RepositoryTypeBucketRepo {
		if removeApp(apps, "jenkins-x/chartmuseum", appsFileName) {
			modified = true
		}
		if addApp(apps, "jenkins-x/bucketrepo", "repositories", appsFileName) {
			modified = true
		}
	}

	/* TODO
	if requirements.Ingress.Kind == config.IngressTypeIstio {
		if removeApp(apps, "stable/nginx-ingress", appsFileName) {
			modified = true
		}
		if addApp(apps, "jx-labs/istio", "jenkins-x/jxboot-helmfile-resources", appsFileName) {
			modified = true
		}
	}
	*/

	if requirements.Cluster.Provider == cloud.KUBERNETES {
		if addApp(apps, "stable/docker-registry", "jenkins-x/jxboot-helmfile-resources", appsFileName) {
			modified = true
		}
	}

	if shouldHaveCertManager(requirements) {
		if addApp(apps, "jetstack/cert-manager", "jenkins-x/jxboot-helmfile-resources", appsFileName) {
			modified = true
		}
		if addApp(apps, "jx-labs/acme", "jenkins-x/jxboot-helmfile-resources", appsFileName) {
			modified = true
		}
		log.Logger().Infof("TLS required, please ensure you have setup any cloud resources as per the documentation //todo docs")
	}

	// add/remove any custom apps from the CLI
	for _, app := range addApps {
		if addApp(apps, app, "repositories", appsFileName) {
			modified = true
		}
	}
	for _, app := range removeApps {
		if removeApp(apps, app, appsFileName) {
			modified = true
		}
	}

	if modified {
		err = apps.SaveConfig(appsFileName)
		if err != nil {
			return apps, appsFileName, errors.Wrapf(err, "failed to save modified file %s", appsFileName)
		}
	}
	return apps, appsFileName, err
}

func shouldHaveCertManager(requirements *config.RequirementsConfig) bool {
	return requirements.Ingress.TLS.Enabled && requirements.Ingress.TLS.SecretName == ""
}

func addApp(apps *jxapps.AppConfig, chartName string, beforeName, appsFileName string) bool {
	idx := -1
	for i, a := range apps.Apps {
		switch a.Name {
		case chartName:
			return false
		case beforeName:
			idx = i
		}
	}
	app := jxapps.App{Name: chartName}

	// if we have a repositories chart lets add apps before that
	if idx >= 0 {
		newApps := append([]jxapps.App{app}, apps.Apps[idx:]...)
		apps.Apps = append(apps.Apps[0:idx], newApps...)
	} else {
		apps.Apps = append(apps.Apps, app)
	}
	log.Logger().Infof("added %s to %s", chartName, appsFileName)
	return true
}

func removeApp(apps *jxapps.AppConfig, chartName, appsFileName string) bool {
	for i, a := range apps.Apps {
		if a.Name == chartName {
			apps.Apps = append(apps.Apps[0:i], apps.Apps[i+1:]...)
			log.Logger().Infof("removed %s to %s", chartName, appsFileName)
			return true
		}
	}
	return false
}

func applyDefaults(cmd *cobra.Command, r *config.RequirementsConfig, flags *RequirementFlags) error {
	// override boolean flags if specified
	if FlagChanged(cmd, "autoupgrade") {
		r.AutoUpdate.Enabled = flags.AutoUpgrade
	}
	if FlagChanged(cmd, "env-git-public") {
		r.Cluster.EnvironmentGitPublic = flags.EnvironmentGitPublic
	}
	if FlagChanged(cmd, "git-public") {
		r.Cluster.GitPublic = flags.GitPublic
	}
	if FlagChanged(cmd, "gitops") {
		r.GitOps = flags.GitOps
	}
	if FlagChanged(cmd, "kaniko") {
		r.Kaniko = flags.Kaniko
	}
	if FlagChanged(cmd, "terraform") {
		r.Terraform = flags.Terraform
	}
	if FlagChanged(cmd, "vault-disable-url-discover") {
		r.Vault.DisableURLDiscovery = flags.VaultDisableURLDiscover
	}
	if FlagChanged(cmd, "vault-recreate-bucket") {
		r.Vault.RecreateBucket = flags.VaultRecreateBucket
	}
	if FlagChanged(cmd, "tls") {
		r.Ingress.TLS.Enabled = flags.TLS
	}
	/* TODO
	if FlagChanged(cmd, "canary") {
		if r.DeployOptions == nil {
			r.DeployOptions = &v1.DeployOptions{}
		}
		r.DeployOptions.Canary = flags.Canary
	}
	if FlagChanged(cmd, "hpa") {
		if r.DeployOptions == nil {
			r.DeployOptions = &v1.DeployOptions{}
		}
		r.DeployOptions.HPA = flags.HPA
	}
	if flags.IngressKind != "" {
		r.Ingress.Kind = config.IngressType(flags.IngressKind)
	}

	*/

	if flags.Repository != "" {
		r.Repository = config.RepositoryType(flags.Repository)
	}
	if flags.SecretStorage != "" {
		r.SecretStorage = config.SecretStorageType(flags.SecretStorage)
	}

	if flags.EnvironmentRemote {
		for i, e := range r.Environments {
			if e.Key == "dev" {
				continue
			}
			r.Environments[i].RemoteCluster = true
		}
	}

	gitKind := r.Cluster.GitKind
	gitKinds := append(gits.KindGits, "fake")
	if gitKind != "" && util.StringArrayIndex(gitKinds, gitKind) < 0 {
		return util.InvalidOption("git-kind", gitKind, gits.KindGits)
	}

	// default flags if associated values
	if r.AutoUpdate.Schedule != "" {
		r.AutoUpdate.Enabled = true
	}
	if r.Ingress.TLS.Email != "" {
		r.Ingress.TLS.Enabled = true
	}

	// enable storage if we specify a URL
	storage := &r.Storage
	if storage.Logs.URL != "" && storage.Reports.URL == "" {
		storage.Reports.URL = storage.Logs.URL
	}
	defaultStorage(&storage.Backup)
	defaultStorage(&storage.Logs)
	defaultStorage(&storage.Reports)
	defaultStorage(&storage.Repository)
	return nil
}

// FlagChanged returns true if the given flag was supplied on the command line
func FlagChanged(cmd *cobra.Command, name string) bool {
	if cmd != nil {
		f := cmd.Flag(name)
		if f != nil {
			return f.Changed
		}
	}
	return false
}

func defaultStorage(storage *config.StorageEntryConfig) {
	if storage.URL != "" {
		storage.Enabled = true
	}
}

// FindRequirementsAndGitURL tries to find the requirements and git URL via either environment or directory
func FindRequirementsAndGitURL(jxFactory jxfactory.Factory, gitURLOption string, gitter gits.Gitter, dir string) (*config.RequirementsConfig, string, error) {
	var requirements *config.RequirementsConfig
	gitURL := gitURLOption

	var err error
	if gitURLOption != "" {
		if requirements == nil {
			requirements, err = GetRequirementsFromGit(gitURL)
			if err != nil {
				return requirements, gitURL, errors.Wrapf(err, "failed to get requirements from git URL %s", gitURL)
			}
		}
	}
	if gitURL == "" || requirements == nil {
		jxClient, ns, err := jxFactory.CreateJXClient()
		if err != nil {
			return requirements, gitURL, err
		}
		devEnv, err := kube.GetDevEnvironment(jxClient, ns)
		if err != nil && !apierrors.IsNotFound(err) {
			return requirements, gitURL, err
		}
		if devEnv != nil {
			if gitURL == "" {
				gitURL = devEnv.Spec.Source.URL
			}
			requirements, err = config.GetRequirementsConfigFromTeamSettings(&devEnv.Spec.TeamSettings)
			if err != nil {
				log.Logger().Debugf("failed to load requirements from team settings %s", err.Error())
			}
		}
	}
	if requirements == nil {
		requirements, _, err = config.LoadRequirementsConfig(dir, false)
		if err != nil {
			return requirements, gitURL, err
		}
	}

	if gitURL == "" {
		// lets try find the git URL from
		gitURL, err = findGitURLFromDir(gitter, dir)
		if err != nil {
			return requirements, gitURL, errors.Wrapf(err, "your cluster has not been booted before and you are not inside a git clone of your dev environment repository so you need to pass in the URL of the git repository as --git-url")
		}
	}
	return requirements, gitURL, nil
}

// FindGitURL tries to find the git URL via either environment or directory
func FindGitURL(jxFactory jxfactory.Factory) (string, error) {
	gitURL := ""
	jxClient, ns, err := jxFactory.CreateJXClient()
	if err != nil {
		return gitURL, err
	}
	devEnv, err := kube.GetDevEnvironment(jxClient, ns)
	if err != nil && !apierrors.IsNotFound(err) {
		return gitURL, err
	}
	if devEnv != nil {
		return devEnv.Spec.Source.URL, nil
	}
	return gitURL, nil
}

func findGitURLFromDir(gitter gits.Gitter, dir string) (string, error) {
	_, gitConfDir, err := gitter.FindGitConfigDir(dir)
	if err != nil {
		return "", errors.Wrapf(err, "there was a problem obtaining the git config dir of directory %s", dir)
	}

	if gitConfDir == "" {
		return "", fmt.Errorf("no .git directory could be found from dir %s", dir)
	}
	return gitter.DiscoverUpstreamGitURL(gitConfDir)
}

// GitKind returns the git kind for the development environment or empty string if it can't be found
func GitKind(devSource v1.EnvironmentRepository, r *config.RequirementsConfig) string {
	answer := string(devSource.Kind)
	if answer == "" {
		if r != nil {
			dev := GetDevEnvironmentConfig(r)
			if dev != nil {
				answer = dev.GitKind
			}
		}
	}
	return answer
}
