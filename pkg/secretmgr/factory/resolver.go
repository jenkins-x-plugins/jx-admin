package factory

import (
	"context"
	"fmt"
	"strings"

	"github.com/jenkins-x/jx-remote/pkg/common"
	"github.com/jenkins-x/jx/pkg/log"

	"github.com/jenkins-x/jx/pkg/util"

	"github.com/jenkins-x/jx/pkg/auth"

	"github.com/jenkins-x/go-scm/scm"
	"github.com/jenkins-x/jx-remote/pkg/authhelpers"

	"github.com/jenkins-x/jx-remote/pkg/reqhelpers"
	"github.com/jenkins-x/jx-remote/pkg/secretmgr"
	v1 "github.com/jenkins-x/jx/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx/pkg/cloud"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/jxfactory"
	"github.com/jenkins-x/jx/pkg/kube"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// KindResolver provides a simple way to resolve what kind of Secret Manager to use
type KindResolver struct {
	Factory           jxfactory.Factory
	Kind              string
	Dir               string
	GitURL            string
	Gitter            gits.Gitter
	AuthConfigService auth.ConfigService
	IoFileHandles     *util.IOFileHandles
	BatchMode         bool
	UseGitHubOauth    bool

	// outputs which can be useful
	DevEnvironment *v1.Environment
	Requirements   *config.RequirementsConfig
}

// CreateScmClient creates a new scm client
func (o *KindResolver) CreateScmClient(gitServer, owner, gitKind string) (*scm.Client, string, string, error) {
	af, err := authhelpers.NewAuthFacadeWithArgs(o.AuthConfigService, o.Git(), o.IoFileHandles, o.BatchMode, o.UseGitHubOauth)
	if err != nil {
		return nil, "", "", errors.Wrapf(err, "failed to create git auth facade")
	}
	scmClient, token, login, err := af.ScmClient(gitServer, owner, gitKind)
	if err != nil {
		return scmClient, "", "", errors.Wrapf(err, "failed to create SCM client for server %s", gitServer)
	}
	return scmClient, token, login, nil
}

// CreateSecretManager detects from the current cluster which kind of SecretManager to use and then creates it
func (r *KindResolver) CreateSecretManager(secretsYAML string) (secretmgr.SecretManager, error) {
	// lets try find the requirements from the cluster or locally
	requirements, ns, err := r.resolveRequirements(secretsYAML)
	if err != nil {
		return nil, err
	}
	r.Requirements = requirements

	if requirements == nil {
		return nil, fmt.Errorf("failed to resolve the jx-requirements.yml from the file system or the 'dev' Environment in namespace %s", ns)
	}
	if r.Kind == "" {
		var err error
		r.Kind, err = r.resolveKind(requirements)
		if err != nil {
			return nil, err
		}

		// if we can't find one default to local Secrets
		if r.Kind == "" {
			r.Kind = secretmgr.KindLocal
		}
	}
	return NewSecretManager(r.Kind, r.GetFactory(), requirements)
}

// GetFactory lazy creates the factory if required
func (r *KindResolver) GetFactory() jxfactory.Factory {
	if r.Factory == nil {
		r.Factory = jxfactory.NewFactory()
	}
	return r.Factory
}

// Git the Gitter for accessing git
func (r *KindResolver) Git() gits.Gitter {
	if r.Gitter == nil {
		r.Gitter = gits.NewGitCLI()
	}
	return r.Gitter
}

// VerifySecrets verifies that the secrets are valid
func (r *KindResolver) VerifySecrets(verifyPipelineUser bool) error {
	secretsYAML := ""
	sm, err := r.CreateSecretManager("")
	if err != nil {
		return err
	}

	cb := func(currentYAML string) (string, error) {
		secretsYAML = currentYAML
		return currentYAML, nil
	}
	err = sm.UpsertSecrets(cb, secretmgr.DefaultSecretsYaml)
	if err != nil {
		return errors.Wrapf(err, "failed to load Secrets YAML from secret manager %s", sm.String())
	}

	secretsYAML = strings.TrimSpace(secretsYAML)
	if secretsYAML == "" {
		return errors.Errorf("empty secrets YAML")
	}

	err = secretmgr.VerifyBootSecrets(secretsYAML)
	if err != nil {
		return errors.Wrapf(err, "failed to verify boot secrets")
	}

	if !verifyPipelineUser {
		return nil
	}
	return r.verifyPipelineUser(secretsYAML)
}

func (r *KindResolver) resolveKind(requirements *config.RequirementsConfig) (string, error) {
	switch requirements.SecretStorage {
	case config.SecretStorageTypeVault:
		return secretmgr.KindVault, nil

	case config.SecretStorageTypeLocal:
		return secretmgr.KindLocal, nil

	case config.SecretStorageTypeGSM:
		if requirements.Cluster.Provider != cloud.GKE {
			return "", fmt.Errorf("google secret manager (GSM) secret store is only supported on the GKE provider")
		}
		return secretmgr.KindGoogleSecretManager, nil
	}
	return secretmgr.KindLocal, nil
}

func (r *KindResolver) resolveRequirements(secretsYAML string) (*config.RequirementsConfig, string, error) {
	jxClient, ns, err := r.GetFactory().CreateJXClient()
	if err != nil {
		return nil, ns, errors.Wrap(err, "failed to create JX Client")
	}

	if r.GitURL == "" {
		r.GitURL, err = r.LoadBootRunGitURLFromSecret()
		if err != nil {
			return nil, "", err
		}
	}

	dev, err := kube.GetDevEnvironment(jxClient, ns)
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, ns, errors.Wrap(err, "failed to find the 'dev' Environment resource")
	}
	r.DevEnvironment = dev
	if r.Requirements != nil {
		return r.Requirements, ns, nil
	}
	if dev != nil {
		if r.GitURL == "" {
			r.GitURL = dev.Spec.Source.URL
		}
		requirements, err := config.GetRequirementsConfigFromTeamSettings(&dev.Spec.TeamSettings)
		if err != nil {
			return nil, ns, errors.Wrapf(err, "failed to unmarshal requirements from 'dev' Environment in namespace %s", ns)
		}
		if requirements != nil {
			return requirements, ns, nil
		}
	}
	if r.GitURL != "" {
		if secretsYAML != "" {
			r.GitURL, err = secretmgr.AddUserTokenToGitURLFromSecretsYAML(r.GitURL, secretsYAML)
			if err != nil {
				return nil, "", errors.Wrap(err, "failed to enrich git URL with user and token from the secrets YAML")
			}
		}
		requirements, err := reqhelpers.GetRequirementsFromGit(r.GitURL)
		return requirements, ns, err
	}

	if r.GitURL == "" {
		// lets try get the git URL from the dir
		r.GitURL, _, err = gits.GetGitInfoFromDirectory(r.Dir, r.Git())
		if err != nil {
			return nil, ns, errors.Wrapf(err, "failed to detect the git URL from the directory %s", r.Dir)
		}
	}

	requirements, _, err := config.LoadRequirementsConfig(r.Dir)
	if err != nil {
		return requirements, ns, errors.Wrapf(err, "failed to requirements YAML file from %s", r.Dir)
	}
	return requirements, ns, nil
}

// SaveBootRunGitCloneSecret saves the git URL used to clone the git repository with the necessary user and token
// so that we can clone private repositories
func (r *KindResolver) SaveBootRunGitCloneSecret(secretsYAML string) error {
	if r.GitURL == "" {
		return fmt.Errorf("no development environment git URL detected")
	}
	var err error
	r.GitURL, err = secretmgr.AddUserTokenToGitURLFromSecretsYAML(r.GitURL, secretsYAML)
	if err != nil {
		return err
	}

	// TODO we could report which secrets are missing so we can provide them better in a UI?
	verifyString := "valid"
	err = r.VerifySecrets(true)
	if err != nil {
		verifyString = fmt.Sprintf("invalid: %s", err.Error())
	}

	kubeClient, ns, err := r.GetFactory().CreateKubeClient()
	if err != nil {
		return errors.Wrap(err, "failed to create Kubernetes client")
	}
	name := secretmgr.BootGitURLSecret
	create := false
	secretInterface := kubeClient.CoreV1().Secrets(ns)
	s, err := secretInterface.Get(name, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return errors.Wrapf(err, "failed to get Secret %s in namespace %s", name, ns)
		}
		create = true
	}
	if s == nil {
		s = &corev1.Secret{}
	}
	s.Name = name
	if s.Data == nil {
		s.Data = map[string][]byte{}
	}
	s.Data[secretmgr.BootGitURLSecretKey] = []byte(r.GitURL)
	s.Data[secretmgr.BootGitURLSecretVerifyKey] = []byte(verifyString)

	if create {
		_, err = secretInterface.Create(s)
		if err != nil {
			return errors.Wrapf(err, "failed to save Git URL secret %s in namespace %s", name, ns)
		}
	} else {
		_, err = secretInterface.Update(s)
		if err != nil {
			return errors.Wrapf(err, "failed to update Git URL secret %s in namespace %s", name, ns)
		}
	}
	return nil
}

// LoadBootRunGitURLFromSecret loads the boot run git clone URL from the secret
func (r *KindResolver) LoadBootRunGitURLFromSecret() (string, error) {
	kubeClient, ns, err := r.GetFactory().CreateKubeClient()
	if err != nil {
		return "", errors.Wrap(err, "failed to create Kubernetes client")
	}
	name := secretmgr.BootGitURLSecret
	key := secretmgr.BootGitURLSecretKey
	s, err := kubeClient.CoreV1().Secrets(ns).Get(name, metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return "", errors.Wrapf(err, "failed to get Secret %s in namespace %s. Please check you setup the boot secrets", name, ns)
		}
	}

	var answer []byte
	if s != nil && s.Data != nil {
		answer = s.Data[key]
	}
	return string(answer), nil
}

func (r *KindResolver) verifyPipelineUser(secretsYAML string) error {
	data := map[string]interface{}{}

	err := yaml.Unmarshal([]byte(secretsYAML), &data)
	if err != nil {
		return errors.Wrap(err, "failed to unmarshal secrets YAML")
	}

	pipelineUser := util.GetMapValueAsStringViaPath(data, "secrets.pipelineUser.username")
	if pipelineUser == "" {
		return errors.Errorf("missing pipeline username in secrets yaml")
	}

	// validate bot user has karma on the the environment git repo, requires admin permissions so boot can register webhooks
	gitInfo, err := gits.ParseGitURL(r.GitURL)
	if err != nil {
		return errors.Wrap(err, "failed to parse git URL")
	}

	serverURL := gitInfo.HostURLWithoutUser()

	owner := gitInfo.Organisation
	repo := scm.Join(owner, gitInfo.Name)
	kind := gits.SaasGitKind(serverURL)

	if kind == "" {
		return fmt.Errorf("no git kind found for url %s", r.GitURL)
	}

	scmClient, _, login, err := r.CreateScmClient(serverURL, owner, kind)
	if err != nil {
		return errors.Wrapf(err, "failed to create scm client")
	}

	// ensure the bot user is different from the operator user else there will be issues with chatops
	if login == pipelineUser {
		return fmt.Errorf("pipeline bot username and git user are the same [%s] they must be different", pipelineUser)
	}

	return r.ensurePipelineUserIsCollaborator(scmClient, owner, repo, pipelineUser)

}

func (r *KindResolver) ensurePipelineUserIsCollaborator(scmClient *scm.Client, owner string, repo, pipelineUser string) error {
	ctx := context.Background()

	// first lets check if pipeline user is an admin at the org level
	isOrgAdmin, _, err := scmClient.Organizations.IsAdmin(ctx, owner, pipelineUser)
	if err != nil {
		return errors.Wrapf(err, "failed to find if pipeline user %s is admin on the organisation %s", pipelineUser, owner)
	}
	if isOrgAdmin {
		log.Logger().Infof("pipeline bot user %s is an admin on the organisation %s so has enough permissions", pipelineUser, owner)
		return nil
	}

	// next lets check pipeline user is a collaborator and has admin permissions
	permission, _, err := scmClient.Repositories.FindUserPermission(ctx, repo, pipelineUser)
	if err != nil {
		log.Logger().Infof("unable to find pipeline bot user %s on repository %s", pipelineUser, repo)
	}

	if err != nil || permission != "admin" {
		log.Logger().Infof("pipeline user %s needs to have admin permissions, it has permission [%s], this is needed for automatically configuring webhooks", pipelineUser, permission)

		if !r.BatchMode {
			ioHandles := common.GetIOFileHandles(r.IoFileHandles)
			c, err := util.Confirm(fmt.Sprintf("Would you like Jenkins X to add the pipeline user [%s] as a collaborator to repository %s giving admin permissions now?", pipelineUser, repo), false, "Destroying your installation will preserve your kubernetes cluster and the underlying cloud resources so you can re-run boot again", ioHandles)
			if err != nil {
				return err
			}
			if !c {
				return fmt.Errorf("please give pipeline user [%s] admin permission on the repository %s or at the organisation level and try again", pipelineUser, repo)
			}

		}

		result, existingCollaborator, _, err := scmClient.Repositories.AddCollaborator(ctx, repo, pipelineUser, "admin")
		if err != nil {
			return errors.Wrapf(err, "failed to add %s as a collaborator to %s", pipelineUser, repo)
		}

		if existingCollaborator {
			log.Logger().Infof("pipeline bot user [%s] already a collaborator on %s, permissions have been updated to admin", pipelineUser, repo)
			return nil
		}
		if !result {
			return fmt.Errorf("pipeline bot user [%s] not added as a collaborator to %s", pipelineUser, repo)
		}
		log.Logger().Infof("please accept invitation on behalf of the pipeline user %s to collaborate on %s", pipelineUser, repo)
	}
	return nil
}
