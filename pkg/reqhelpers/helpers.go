package reqhelpers

import (
	"fmt"
	"os"

	"github.com/jenkins-x/jx-admin/pkg/common"
	v1 "github.com/jenkins-x/jx-api/v3/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx-api/v3/pkg/config"
	"github.com/jenkins-x/jx-helpers/v3/pkg/files"
	"github.com/jenkins-x/jx-helpers/v3/pkg/gitclient/giturl"
	"github.com/jenkins-x/jx-helpers/v3/pkg/kube/naming"
	"github.com/jenkins-x/jx-helpers/v3/pkg/options"
	"github.com/jenkins-x/jx-helpers/v3/pkg/stringhelpers"
	"github.com/jenkins-x/jx-helpers/v3/pkg/termcolor"
	"github.com/jenkins-x/jx-logging/v3/pkg/log"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
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
