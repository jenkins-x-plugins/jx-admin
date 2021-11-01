package reqhelpers

import (
	"fmt"
	"os"

	jxcore "github.com/jenkins-x/jx-api/v4/pkg/apis/core/v4beta1"
	v1 "github.com/jenkins-x/jx-api/v4/pkg/apis/jenkins.io/v1"
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
	TLS                                                             bool
	Canary, HPA                                                     bool
	VaultRecreateBucket, VaultDisableURLDiscover                    bool
	LogsURL                                                         string
	BackupsURL                                                      string
	ReportsURL                                                      string
	RepositoryURL                                                   string
}

// GetDevEnvironmentConfig returns the dev environment for the given requirements or nil
func GetDevEnvironmentConfig(requirements *jxcore.RequirementsConfig) *jxcore.EnvironmentConfig {
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
	outputRequirements *jxcore.RequirementsConfig, flags *RequirementFlags, environment string) error {
	requirementsResource, fileName, err := jxcore.LoadRequirementsConfig(dir, false)
	if err != nil {
		return err
	}
	requirements := &requirementsResource.Spec
	if customRequirementsFile != "" {
		exists, err := files.FileExists(customRequirementsFile)
		if err != nil {
			return errors.Wrapf(err, "failed to check if file exists: %s", customRequirementsFile)
		}
		if !exists {
			return fmt.Errorf("custom requirements file %s does not exist", customRequirementsFile)
		}
		requirementsResource, err = jxcore.LoadRequirementsConfigFile(customRequirementsFile, false)
		if err != nil {
			return errors.Wrapf(err, "failed to load: %s", customRequirementsFile)
		}

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

	outputRequirements, err = applyDefaults(cmd, outputRequirements, flags)
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
			outputRequirements.Environments = append(outputRequirements.Environments, jxcore.EnvironmentConfig{
				Key:               "dev",
				Owner:             outputRequirements.Cluster.EnvironmentGitOwner,
				Repository:        repository,
				GitServer:         outputRequirements.Cluster.GitServer,
				GitKind:           outputRequirements.Cluster.GitKind,
				RemoteCluster:     true,
				PromotionStrategy: promoteStrategy,
			})
		}
		outputRequirements.Webhook = jxcore.WebhookTypeLighthouse
	}

	// lets default an ingress service type if there is none
	/* TODO
	if outputRequirements.Ingress.ServiceType == "" {
		outputRequirements.Ingress.ServiceType = "LoadBalancer"
	}
	*/

	if string(outputRequirements.SecretStorage) == "" {
		outputRequirements.SecretStorage = jxcore.SecretStorageTypeLocal
	}

	requirementsResource.Spec = *outputRequirements
	err = requirementsResource.SaveConfig(fileName)
	if err != nil {
		return errors.Wrapf(err, "failed to save %s", fileName)
	}

	log.Logger().Infof("saved file: %s", termcolor.ColorInfo(fileName))
	return nil
}

func applyDefaults(cmd *cobra.Command, r *jxcore.RequirementsConfig, flags *RequirementFlags) (*jxcore.RequirementsConfig, error) {
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
		r.Repository = jxcore.RepositoryType(flags.Repository)
	}
	if flags.SecretStorage != "" {
		r.SecretStorage = jxcore.SecretStorageType(flags.SecretStorage)
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
	giturl.KindGits = append(giturl.KindGits, "fake")
	if gitKind != "" && stringhelpers.StringArrayIndex(giturl.KindGits, gitKind) < 0 {
		return nil, options.InvalidOption("git-kind", gitKind, giturl.KindGits)
	}

	// default flags if associated values
	if r.AutoUpdate.Schedule != "" {
		r.AutoUpdate.Enabled = true
	}
	if r.Ingress.TLS != nil && r.Ingress.TLS.Email != "" {
		r.Ingress.TLS.Enabled = true
	}

	// enable storage if we specify a URL
	if r.GetStorageURL("logs") != "" && r.GetStorageURL("reports") == "" {
		r.AddOrUpdateStorageURL("reports", r.GetStorageURL("logs"))
	}
	if flags.LogsURL != "" {
		r.AddOrUpdateStorageURL("logs", flags.LogsURL)
	}
	if flags.BackupsURL != "" {
		r.AddOrUpdateStorageURL("backup", flags.BackupsURL)
	}
	if flags.ReportsURL != "" {
		r.AddOrUpdateStorageURL("reports", flags.ReportsURL)
	}
	if flags.RepositoryURL != "" {
		r.AddOrUpdateStorageURL("repository", flags.RepositoryURL)
	}
	return r, nil
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

// GitKind returns the git kind for the development environment or empty string if it can't be found
func GitKind(devSource v1.EnvironmentRepository, r *jxcore.RequirementsConfig) string {
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
