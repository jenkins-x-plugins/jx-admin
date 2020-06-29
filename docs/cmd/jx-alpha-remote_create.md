## jx-alpha-remote create

Creates a new git repository for a new Jenkins X installation

### Usage

```
jx-alpha-remote create
```

### Synopsis

Creates a new git repository for a new Jenkins X installation

### Examples

  # create a new git repository which we can then boot up
  jx-alpha-remote create

### Options

```
      --add jx-apps.yml              The apps/charts to add to the jx-apps.yml file to add the apps
      --approver stringArray         the git user names of the approvers for the environments
      --autoupdate-schedule string   the cron schedule for auto upgrading your cluster
      --autoupgrade                  enables or disables auto upgrades
  -b, --batch-mode                   Enables batch mode which avoids prompting for user input
      --boot-config-url string       specify the boot configuration git URL
      --bucket-backups string        the bucket URL to store backups
      --bucket-logs string           the bucket URL to store logs
      --bucket-repo string           the bucket URL to store repository artifacts
      --bucket-reports string        the bucket URL to store reports. If not specified default to te logs bucket
      --canary                       enables Canary deployment of apps by default
  -c, --cluster string               configures the cluster name
      --dev-git-kind string          The kind of git server for the development environment
      --dev-git-url string           The git URL of the development environment if you are creating a remote staging/production cluster. If specified this will create a Pull Request on the development cluster
      --dir string                   The directory used to create the development environment git repository inside. If not specified a temporary directory will be used
  -d, --domain string                configures the domain name
  -e, --env string                   The name of the remote environment to create
      --env-git-owner string         the git owner (organisation or user) used to own the git repositories for the environments
      --env-git-public               enables or disables whether the environment repositories should be public
      --env-remote                   if enables then all other environments than dev (staging & production by default) will be configured to be in remote clusters
      --extdns-sa string             configures the External DNS service account name
      --git-kind string              the kind of git repository to use. Possible values: bitbucketcloud, bitbucketserver, gitea, github, gitlab
      --git-name string              the name of the git repository
      --git-public                   enables or disables whether the project repositories should be public
      --git-server string            the git server host such as https://github.com or https://gitlab.com
  -g, --gitops                       enables or disables the use of gitops
  -h, --help                         help for create
      --hpa                          enables HPA deployment of apps by default
      --ingress-kind string          configures the kind of ingress used (e.g. whether to use Ingress or VirtualService resources. Possible values: ingress, istio
      --ingress-namespace string     configures the service kind. e.g. specify NodePort unless you want to use the default LoadBalancer
      --ingress-service string       configures the ingress service name when no ingress domain is specified and we need to detect the LoadBalancer IP
      --initial-git-url string       The git URL to clone to fetch the initial set of files for a helm 3 / helmfile based git configuration if this command is not run inside a git clone or against a GitOps based cluster
      --kaniko                       enables or disables the use of kaniko
      --kaniko-sa string             configures the Kaniko service account name
  -n, --namespace string             configures the namespace to use
      --oauth                        Enables the use of OAuth login to github.com to get a github access token
      --out string                   the name of the file to save with the created git URL inside
      --project string               configures the Google Project ID
  -p, --provider string              configures the kubernetes provider.  Supported providers: aks, alibaba, aws, eks, gke, icp, iks, jx-infra, kind, kubernetes, oke, openshift, pks
      --region string                configures the cloud region
      --registry string              configures the host name of the container registry
      --remove jx-apps.yml           The apps/charts to remove from the jx-apps.yml file to remove the apps
      --repo string                  the name of the development git repository to create
      --repository string            the artifact repository. Possible values are: none, bucketrepo, nexus, artifactory
  -r, --requirements string          The 'jx-requirements.yml' file to use in the created development git repository. This file may be created via terraform
      --secret string                configures the secret storage kind. Possible values: gsm, local, vault
      --service-type string          the Ingress controller Service Type such as NodePort if using on premise and you do not have a LoadBalancer service type support
      --terraform                    enables or disables the use of terraform
      --tls                          enable TLS for Ingress
      --tls-email string             the TLS email address to enable TLS on the domain
      --tls-production               the LetsEncrypt production service, defaults to true, set to false to use the Staging service (default true)
      --tls-secret string            [optional] the custom Kubernetes Secret name for the TLS certificate
      --vault-bucket string          specify the vault bucket
      --vault-disable-url-discover   override the default lookup of the Vault URL, could be incluster service or external ingress
      --vault-key string             specify the vault key
      --vault-keyring string         specify the vault key ring
      --vault-name string            specify the vault name
      --vault-recreate-bucket        enables or disables whether to rereate the secret bucket on boot
      --vault-sa string              specify the vault Service Account name
      --velero-ns string             specify the Velero Namespace
      --velero-sa string             specify the Velero Service Account name
      --version-stream-ref string    specify the Version Stream git reference (branch, tag, sha)
      --version-stream-url string    specify the Version Stream git URL
  -z, --zone string                  configures the cloud zone
```

### SEE ALSO

* [jx-alpha-remote](jx-alpha-remote.md)	 - boots up Jenkins and/or Jenkins X in a Kubernetes cluster using GitOps

###### Auto generated by spf13/cobra on 29-Jun-2020
