package common

const (
	// DefaultBootHelmfileRepository default git repo for boot with helmfile
	DefaultBootHelmfileRepository = "https://github.com/jenkins-x/jxr-boot-config.git"

	// DefaultEnvironmentHelmfileGitRepoURL the default git repository used for remote environments with helmfile
	DefaultEnvironmentHelmfileGitRepoURL = "https://github.com/jenkins-x/jxr-default-environment-helmfile.git"

	// DefaultVersionsRef default version stream ref
	DefaultVersionsRef = "master"

	// DefaultVersionsURL default version stream url
	DefaultVersionsURL = "https://github.com/jenkins-x/jxr-versions.git"

	// HelmfileBuildPackName the build pack name for helm 3 / helmfile style environments
	HelmfileBuildPackName = "environment-helmfile"

	// PipelineActivitiesYAMLFile the name of the YAML file to help migrate PipelineActivity resources to a new cluster
	PipelineActivitiesYAMLFile = "pipelineActivities.yaml"
)
