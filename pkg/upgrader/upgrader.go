package upgrader

import (
	"github.com/jenkins-x/jx-remote/pkg/reqhelpers"
	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/versionstream"
)

// HelmfileUpgrader moves an existing cluster to the new helmfile / helm 3 GitOps source
type HelmfileUpgrader struct {
	// Environments the environments to use
	Environments []v1.Environment

	// Requirements the installation requirements
	Requirements *config.RequirementsConfig

	// OverrideRequirements allows custom overrides to git repository/kinds etc
	OverrideRequirements *config.RequirementsConfig

	// VersionResolver the resolver of versions in the version stream
	VersionResolver *versionstream.VersionResolver

	// DevSource the source  the development environment
	DevSource v1.EnvironmentRepository
}

// ExportRequirements generates the exported requirements given the current cluster if there is no environments file
func (u *HelmfileUpgrader) ExportRequirements() (*config.RequirementsConfig, error) {
	answer := config.NewRequirementsConfig()

	// lets default the requirements first
	for _, e := range u.Environments {
		if e.Name == "dev" {
			if u.DevSource.URL == "" {
				u.DevSource = e.Spec.Source
			}
			config, err := config.GetRequirementsConfigFromTeamSettings(&e.Spec.TeamSettings)
			if err == nil && config != nil {
				answer = config

				// lets populate the input requirements so we can default the environment git values
				if u.Requirements == nil {
					u.Requirements = answer
				}
			}
		}
	}
	reqhelpers.UpgradeExistingRequirements(answer)

	// if the environment git owner is missing lets default it from the dev team settings
	if answer.Cluster.EnvironmentGitOwner == "" {
		for _, e := range u.Environments {
			if e.Name == "dev" {
				answer.Cluster.EnvironmentGitOwner = e.Spec.TeamSettings.EnvOrganisation
				break
			}
		}
	}

	for _, e := range u.Environments {
		env := u.GetOrCreateEnvironment(&e, answer)
		env.PromotionStrategy = e.Spec.PromotionStrategy
	}

	return u.overrideRequirements(answer)
}

func (u *HelmfileUpgrader) GetOrCreateEnvironment(e *v1.Environment, requirements *config.RequirementsConfig) *config.EnvironmentConfig {
	inputRequirements := u.Requirements
	if inputRequirements == nil {
		inputRequirements = config.NewRequirementsConfig()
	}
	clusterConfig := inputRequirements.Cluster
	gitOwner := clusterConfig.EnvironmentGitOwner
	gitRepository := ""
	gitServer := clusterConfig.GitServer
	gitKind := clusterConfig.GitKind
	gitURL := e.Spec.Source.URL

	if gitURL != "" {
		gitInfo, err := gits.ParseGitURL(gitURL)
		if err != nil {
			return nil
		}
		if gitInfo != nil {
			if gitInfo.Organisation != "" {
				gitOwner = gitInfo.Organisation
			}
			if gitInfo.Name != "" {
				gitRepository = gitInfo.Name
			}
			if gitInfo.Host != "" && gitServer == "" {
				gitServer = gitInfo.Host
			}
		}
	}

	for i := range requirements.Environments {
		env := &requirements.Environments[i]
		if env.Key == e.Name {
			if env.Repository == "" {
				env.Repository = gitRepository
			}
			if env.Owner == "" {
				env.Owner = gitOwner
			}
			if env.GitServer == "" {
				env.GitServer = gitServer
			}
			if env.GitKind == "" {
				env.GitKind = gitKind
			}
			return env
		}
	}
	env := config.EnvironmentConfig{
		Key:               e.Name,
		Owner:             gitOwner,
		Repository:        gitRepository,
		GitServer:         gitServer,
		GitKind:           gitKind,
		RemoteCluster:     e.Spec.RemoteCluster,
		PromotionStrategy: e.Spec.PromotionStrategy,
	}
	requirements.Environments = append(requirements.Environments, env)
	return &env
}

func (u *HelmfileUpgrader) overrideRequirements(answer *config.RequirementsConfig) (*config.RequirementsConfig, error) {
	o := u.OverrideRequirements
	if o != nil {
		c := o.Cluster
		if c.EnvironmentGitOwner != "" {
			answer.Cluster.EnvironmentGitOwner = c.EnvironmentGitOwner
		}
		if c.GitKind != "" {
			answer.Cluster.GitKind = c.GitKind
		}
		if c.GitName != "" {
			answer.Cluster.GitName = c.GitName
		}
		if c.GitServer != "" {
			answer.Cluster.GitServer = c.GitServer
		}
	}
	return answer, nil
}
