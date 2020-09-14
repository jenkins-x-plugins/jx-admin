package reqhelpers

import (
	"fmt"
	"os"

	"github.com/jenkins-x/jx-admin/pkg/common"
	v1 "github.com/jenkins-x/jx-api/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx-api/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx-api/pkg/config"
	"github.com/jenkins-x/jx-helpers/pkg/cmdrunner"
	"github.com/jenkins-x/jx-helpers/pkg/files"
	"github.com/jenkins-x/jx-helpers/pkg/gitclient/giturl"
	"github.com/jenkins-x/jx-helpers/pkg/kube/jxenv"
	"github.com/jenkins-x/jx-helpers/pkg/kube/naming"
	"github.com/jenkins-x/jx-helpers/pkg/options"
	"github.com/jenkins-x/jx-helpers/pkg/stringhelpers"
	"github.com/jenkins-x/jx-helpers/pkg/termcolor"
	"github.com/jenkins-x/jx-logging/pkg/log"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
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
	for k := range requirements.Environments {
		e := requirements.Environments[k]
		if e.Key == "dev" {
			return &requirements.Environments[k]
		}
	}
	return nil
}

// GetBootJobCommand returns the boot job command
func GetBootJobCommand(requirements *config.RequirementsConfig, gitURL, gitUser, gitToken, chartName, version, repo, tag, helmBin string, secretsYAML bool) cmdrunner.Command {
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

	return cmdrunner.Command{
		Name: helmBin,
		Args: args,
	}
}

// GetRequirementsFromEnvironment tries to find the development environment then the requirements from it
func GetRequirementsFromEnvironment(kubeClient kubernetes.Interface, jxClient versioned.Interface, namespace string) (*v1.Environment, *config.RequirementsConfig, error) {
	ns, _, err := jxenv.GetDevNamespace(kubeClient, namespace)
	if err != nil {
		log.Logger().Warnf("could not find the dev namespace from namespace %s due to %s", namespace, err.Error())
		ns = namespace
	}
	devEnv, err := jxenv.GetDevEnvironment(jxClient, ns)
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
		devEnv = jxenv.CreateDefaultDevEnvironment(ns)
	}
	if devEnv != nil && devEnv.Namespace == "" {
		devEnv.Namespace = ns
	}
	requirements := config.NewRequirementsConfig()
	return devEnv, requirements, nil
}

// OverrideRequirements allows CLI overrides
func OverrideRequirements(cmd *cobra.Command, args []string, dir, customRequirementsFile string,
	outputRequirements *config.RequirementsConfig, flags *RequirementFlags, environment string) error {
	requirements, fileName, err := config.LoadRequirementsConfig(dir, false)
	if err != nil {
		return err
	}

	if customRequirementsFile != "" {
		exists, err := files.FileExists(customRequirementsFile)
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

	log.Logger().Infof("saved file: %s", termcolor.ColorInfo(fileName))
	return nil
}

// UpgradeExistingRequirements updates a custom requirements file for helm 3
func UpgradeExistingRequirements(requirements *config.RequirementsConfig) {
	requirements.GitOps = true
	requirements.Helmfile = true
	requirements.Webhook = config.WebhookTypeLighthouse
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
		for k := range r.Environments {
			e := r.Environments[k]
			if e.Key == "dev" {
				continue
			}
			r.Environments[k].RemoteCluster = true
		}
	}

	gitKind := r.Cluster.GitKind
	gitKinds := append(giturl.KindGits, "fake")
	if gitKind != "" && stringhelpers.StringArrayIndex(gitKinds, gitKind) < 0 {
		return options.InvalidOption("git-kind", gitKind, giturl.KindGits)
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

/* TODO
// FindRequirementsAndGitURL tries to find the requirements and git URL via either environment or directory
func FindRequirementsAndGitURL(jxFactory jxfactory.Factory, gitURLOption string, gitter gitclient.Interface, dir string) (*config.RequirementsConfig, string, error) {
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
		devEnv, err := jxenv.GetDevEnvironment(jxClient, ns)
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
		gitURL, err = findGitURLFromDir(dir)
		if err != nil {
			return requirements, gitURL, errors.Wrapf(err, "your cluster has not been booted before and you are not inside a git clone of your dev environment repository so you need to pass in the URL of the git repository as --git-url")
		}
	}
	return requirements, gitURL, nil
}


// FindGitURL tries to find the git URL via either environment or directory
func FindGitURL(jxClient versioned.Interface) (string, error) {
	gitURL := ""
	jxClient, ns, err := jxFactory.CreateJXClient()
	if err != nil {
		return gitURL, err
	}
	devEnv, err := jxenv.GetDevEnvironment(jxClient, ns)
	if err != nil && !apierrors.IsNotFound(err) {
		return gitURL, err
	}
	if devEnv != nil {
		return devEnv.Spec.Source.URL, nil
	}
	return gitURL, nil
}

func findGitURLFromDir(dir string) (string, error) {
	_, gitConfDir, err := gitclient.FindGitConfigDir(dir)
	if err != nil {
		return "", errors.Wrapf(err, "there was a problem obtaining the git config dir of directory %s", dir)
	}

	if gitConfDir == "" {
		return "", fmt.Errorf("no .git directory could be found from dir %s", dir)
	}
	return gitconfig.DiscoverUpstreamGitURL(gitConfDir)
}
*/

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
