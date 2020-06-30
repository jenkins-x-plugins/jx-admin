package reqhelpers

import (
	"fmt"
	"strings"

	"github.com/jenkins-x/jx-api/pkg/config"
	"github.com/jenkins-x/jx/v2/pkg/cloud"
	"github.com/jenkins-x/jx/v2/pkg/gits"
	"github.com/spf13/cobra"
)

// AddRequirementsFlagsOptions add CLI options to the flags
func AddRequirementsFlagsOptions(cmd *cobra.Command, flags *RequirementFlags) {
	cmd.Flags().BoolVarP(&flags.AutoUpgrade, "autoupgrade", "", false, "enables or disables auto upgrades")
	cmd.Flags().BoolVarP(&flags.EnvironmentRemote, "env-remote", "", false, "if enables then all other environments than dev (staging & production by default) will be configured to be in remote clusters")
	cmd.Flags().BoolVarP(&flags.EnvironmentGitPublic, "env-git-public", "", false, "enables or disables whether the environment repositories should be public")
	cmd.Flags().BoolVarP(&flags.GitPublic, "git-public", "", false, "enables or disables whether the project repositories should be public")
	cmd.Flags().BoolVarP(&flags.GitOps, "gitops", "g", false, "enables or disables the use of gitops")
	cmd.Flags().BoolVarP(&flags.Kaniko, "kaniko", "", false, "enables or disables the use of kaniko")
	cmd.Flags().BoolVarP(&flags.Terraform, "terraform", "", false, "enables or disables the use of terraform")
	cmd.Flags().BoolVarP(&flags.VaultRecreateBucket, "vault-recreate-bucket", "", false, "enables or disables whether to rereate the secret bucket on boot")
	cmd.Flags().BoolVarP(&flags.VaultDisableURLDiscover, "vault-disable-url-discover", "", false, "override the default lookup of the Vault URL, could be incluster service or external ingress")
	cmd.Flags().BoolVarP(&flags.Canary, "canary", "", false, "enables Canary deployment of apps by default")
	cmd.Flags().BoolVarP(&flags.HPA, "hpa", "", false, "enables HPA deployment of apps by default")
	cmd.Flags().BoolVarP(&flags.TLS, "tls", "", false, "enable TLS for Ingress")
	cmd.Flags().StringVarP(&flags.Repository, "repository", "", "", "the artifact repository. Possible values are: "+strings.Join(config.RepositoryTypeValues, ", "))
	cmd.Flags().StringVarP(&flags.SecretStorage, "secret", "", "", "configures the secret storage kind. Possible values: "+strings.Join(config.SecretStorageTypeValues, ", "))
}

// AddRequirementsOptions add CLI flags to the requirements
func AddRequirementsOptions(cmd *cobra.Command, r *config.RequirementsConfig) {
	cmd.Flags().StringVarP(&r.BootConfigURL, "boot-config-url", "", "", "specify the boot configuration git URL")

	// auto upgrade
	cmd.Flags().StringVarP(&r.AutoUpdate.Schedule, "autoupdate-schedule", "", "", "the cron schedule for auto upgrading your cluster")

	// cluster
	cmd.Flags().StringVarP(&r.Cluster.ClusterName, "cluster", "c", "", "configures the cluster name")
	cmd.Flags().StringVarP(&r.Cluster.Namespace, "namespace", "n", "", "configures the namespace to use")
	cmd.Flags().StringVarP(&r.Cluster.Provider, "provider", "p", "", "configures the kubernetes provider.  Supported providers: "+cloud.KubernetesProviderOptions())
	cmd.Flags().StringVarP(&r.Cluster.ProjectID, "project", "", "", "configures the Google Project ID")
	cmd.Flags().StringVarP(&r.Cluster.Registry, "registry", "", "", "configures the host name of the container registry")
	cmd.Flags().StringVarP(&r.Cluster.Region, "region", "", "", "configures the cloud region")
	cmd.Flags().StringVarP(&r.Cluster.Zone, "zone", "z", "", "configures the cloud zone")

	cmd.Flags().StringVarP(&r.Cluster.ExternalDNSSAName, "extdns-sa", "", "", "configures the External DNS service account name")
	cmd.Flags().StringVarP(&r.Cluster.KanikoSAName, "kaniko-sa", "", "", "configures the Kaniko service account name")

	AddGitRequirementsOptions(cmd, r)

	// ingress
	cmd.Flags().StringVarP(&r.Ingress.Domain, "domain", "d", "", "configures the domain name")
	cmd.Flags().StringVarP(&r.Ingress.TLS.Email, "tls-email", "", "", "the TLS email address to enable TLS on the domain")
	cmd.Flags().BoolVarP(&r.Ingress.TLS.Production, "tls-production", "", true, "the LetsEncrypt production service, defaults to true, set to false to use the Staging service")
	cmd.Flags().StringVarP(&r.Ingress.TLS.SecretName, "tls-secret", "", "", "[optional] the custom Kubernetes Secret name for the TLS certificate")

	// storage
	cmd.Flags().StringVarP(&r.Storage.Logs.URL, "bucket-logs", "", "", "the bucket URL to store logs")
	cmd.Flags().StringVarP(&r.Storage.Backup.URL, "bucket-backups", "", "", "the bucket URL to store backups")
	cmd.Flags().StringVarP(&r.Storage.Repository.URL, "bucket-repo", "", "", "the bucket URL to store repository artifacts")
	cmd.Flags().StringVarP(&r.Storage.Reports.URL, "bucket-reports", "", "", "the bucket URL to store reports. If not specified default to te logs bucket")

	// vault
	cmd.Flags().StringVarP(&r.Vault.Name, "vault-name", "", "", "specify the vault name")
	cmd.Flags().StringVarP(&r.Vault.Bucket, "vault-bucket", "", "", "specify the vault bucket")
	cmd.Flags().StringVarP(&r.Vault.Keyring, "vault-keyring", "", "", "specify the vault key ring")
	cmd.Flags().StringVarP(&r.Vault.Key, "vault-key", "", "", "specify the vault key")
	cmd.Flags().StringVarP(&r.Vault.ServiceAccount, "vault-sa", "", "", "specify the vault Service Account name")

	// velero
	cmd.Flags().StringVarP(&r.Velero.ServiceAccount, "velero-sa", "", "", "specify the Velero Service Account name")
	cmd.Flags().StringVarP(&r.Velero.Namespace, "velero-ns", "", "", "specify the Velero Namespace")

	// version stream
	cmd.Flags().StringVarP(&r.VersionStream.URL, "version-stream-url", "", "", "specify the Version Stream git URL")
	cmd.Flags().StringVarP(&r.VersionStream.Ref, "version-stream-ref", "", "", "specify the Version Stream git reference (branch, tag, sha)")
}

// AddGitRequirementsOptions adds git specific overrides to the given requirements
func AddGitRequirementsOptions(cmd *cobra.Command, r *config.RequirementsConfig) {
	cmd.Flags().StringVarP(&r.Cluster.GitKind, "git-kind", "", "", fmt.Sprintf("the kind of git repository to use. Possible values: %s", strings.Join(gits.KindGits, ", ")))
	cmd.Flags().StringVarP(&r.Cluster.GitName, "git-name", "", "", "the name of the git repository")
	cmd.Flags().StringVarP(&r.Cluster.GitServer, "git-server", "", "", "the git server host such as https://github.com or https://gitlab.com")
	cmd.Flags().StringVarP(&r.Cluster.EnvironmentGitOwner, "env-git-owner", "", "", "the git owner (organisation or user) used to own the git repositories for the environments")
	cmd.Flags().StringArrayVarP(&r.Cluster.DevEnvApprovers, "approver", "", nil, "the git user names of the approvers for the environments")
}
