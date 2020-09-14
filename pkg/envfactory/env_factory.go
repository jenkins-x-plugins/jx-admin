package envfactory

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/jenkins-x/go-scm/scm"
	"github.com/jenkins-x/jx-admin/pkg/common"
	"github.com/jenkins-x/jx-admin/pkg/reqhelpers"
	"github.com/jenkins-x/jx-api/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx-api/pkg/config"
	"github.com/jenkins-x/jx-helpers/pkg/cmdrunner"
	"github.com/jenkins-x/jx-helpers/pkg/files"
	"github.com/jenkins-x/jx-helpers/pkg/gitclient"
	"github.com/jenkins-x/jx-helpers/pkg/gitclient/giturl"
	"github.com/jenkins-x/jx-helpers/pkg/input"
	"github.com/jenkins-x/jx-helpers/pkg/input/survey"
	"github.com/jenkins-x/jx-helpers/pkg/scmhelpers"
	"github.com/jenkins-x/jx-helpers/pkg/termcolor"
	"github.com/jenkins-x/jx-logging/pkg/log"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes"
)

type EnvFactory struct {
	KubeClient           kubernetes.Interface
	JXClient             versioned.Interface
	Gitter               gitclient.Interface
	CommandRunner        cmdrunner.CommandRunner
	ScmClientFactory     scmhelpers.Factory
	Input                input.Interface
	RepoName             string
	GitURLOutFile        string
	OutDir               string
	IOFileHandles        *files.IOFileHandles
	ScmClient            *scm.Client
	BatchMode            bool
	CreatedRepository    *scmhelpers.CreateRepository
	CreatedScmRepository *scm.Repository
}

// AddFlags adds common CLI flags
func (o *EnvFactory) AddFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(&o.BatchMode, "batch-mode", "b", false, "Enables batch mode which avoids prompting for user input")
	cmd.Flags().StringVarP(&o.RepoName, "repo", "", "", "the name of the development git repository to create")
	cmd.Flags().StringVarP(&o.GitURLOutFile, "out", "", "", "the name of the file to save with the created git URL inside")
	cmd.Flags().StringVarP(&o.ScmClientFactory.GitToken, "git-token", "", "", "the git token used to operate on the git repository")
}

// CreateDevEnvGitRepository creates the dev environment git repository from the given directory
func (o *EnvFactory) CreateDevEnvGitRepository(dir string, gitPublic bool) error {
	o.OutDir = dir
	requirements, fileName, err := config.LoadRequirementsConfig(dir, false)
	if err != nil {
		return errors.Wrapf(err, "failed to load requirements from %s", dir)
	}

	dev := reqhelpers.GetDevEnvironmentConfig(requirements)
	if dev == nil {
		return fmt.Errorf("the file %s does not contain a development environment", fileName)
	}

	cr := &scmhelpers.CreateRepository{
		GitServer:  requirements.Cluster.GitServer,
		GitKind:    requirements.Cluster.GitKind,
		Owner:      dev.Owner,
		Repository: dev.Repository,
		GitPublic:  gitPublic,
	}
	if cr.Owner == "" {
		cr.Owner = requirements.Cluster.EnvironmentGitOwner
	}
	if cr.Repository == "" {
		cr.Repository = o.RepoName
	}
	o.CreatedRepository = cr
	err = cr.ConfirmValues(o.GetInput(), o.BatchMode)
	if err != nil {
		return err
	}

	scmClient, _, err := o.CreateScmClient(cr.GitServer, cr.Owner, cr.GitKind)
	if err != nil {
		return errors.Wrapf(err, "failed to create SCM client for server %s", cr.GitServer)
	}
	o.ScmClient = scmClient

	repo, err := cr.CreateRepository(scmClient)
	if err != nil {
		return err
	}
	o.CreatedScmRepository = repo
	err = o.PushToGit(repo.Clone, dir)
	if err != nil {
		return errors.Wrap(err, "failed to push to the git repository")
	}
	err = o.PrintBootJobInstructions(requirements, repo.Link)
	if err != nil {
		return err
	}
	if o.GitURLOutFile != "" {
		err = ioutil.WriteFile(o.GitURLOutFile, []byte(repo.Link), files.DefaultFileWritePermissions)
		if err != nil {
			return errors.Wrapf(err, "failed to save Git URL to file %s", o.GitURLOutFile)
		}
	}
	return nil
}

// GetInput lazily creates the input interface
func (o *EnvFactory) GetInput() input.Interface {
	if o.Input == nil {
		o.Input = survey.NewInput()
	}
	return o.Input
}

// CreateScmClient creates a new scm client
func (o *EnvFactory) CreateScmClient(gitServer, owner, gitKind string) (*scm.Client, string, error) {
	o.ScmClientFactory.GitServerURL = gitServer
	o.ScmClientFactory.Owner = owner
	o.ScmClientFactory.GitKind = gitKind
	scmClient, err := o.ScmClientFactory.Create()
	if err != nil {
		return scmClient, "", errors.Wrapf(err, "failed to create SCM client for server %s", gitServer)
	}
	return scmClient, o.ScmClientFactory.GitToken, nil
}

// VerifyPreInstall verify the pre install of boot
func (o *EnvFactory) VerifyPreInstall(disableVerifyPackages bool, dir string) error {
	/*
		vo := verify.StepVerifyPreInstallOptions{}
		vo.CommonOptions = o.JXAdapter().NewCommonOptions()
		vo.Dir = dir
		vo.DisableVerifyPackages = disableVerifyPackages
		vo.NoSecretYAMLValidate = true
		return vo.Run()        mb
	*/

	// TODO invoke the jx CLI?
	return nil
}

// PrintBootJobInstructions prints the instructions to run the installer
func (o *EnvFactory) PrintBootJobInstructions(requirements *config.RequirementsConfig, link string) error {
	gitInfo, err := giturl.ParseGitURL(link)
	if err != nil {
		return errors.Wrapf(err, "failed to parse git URL %s", link)
	}

	info := termcolor.ColorInfo
	log.Logger().Info("\nto boot your cluster run the following commands:\n\n")

	log.Logger().Infof("%s", info(fmt.Sprintf("git clone %s", link)))
	log.Logger().Infof("%s", info(fmt.Sprintf("cd %s", gitInfo.Name)))
	log.Logger().Infof("%s", info(fmt.Sprintf("%s admin operator", common.BinaryName)))
	log.Logger().Infof("\n\n")
	return nil
}

// PushToGit pushes to the git repository
func (o *EnvFactory) PushToGit(cloneURL string, dir string) error {
	forkPushURL, err := o.ScmClientFactory.CreateAuthenticatedURL(cloneURL)
	if err != nil {
		return errors.Wrapf(err, "creating push URL for %s", cloneURL)
	}

	remoteBranch := "master"
	_, err = o.Gitter.Command(dir, "push", forkPushURL, "--force", fmt.Sprintf("%s:%s", "HEAD", remoteBranch))
	if err != nil {
		return errors.Wrapf(err, "pushing merged branch %s", remoteBranch)
	}

	log.Logger().Infof("pushed code to the repository")
	return nil
}

// CreatePullRequest crates a pull request if there are git changes
func (o *EnvFactory) CreatePullRequest(dir, gitURL, kind, branchName, commitTitle, commitBody string) error {
	if gitURL == "" {
		log.Logger().Infof("no git URL specified so cannot create a Pull Request. Changes have been saved to %s", dir)
		return nil
	}

	gitter := o.Gitter
	changes, err := gitclient.HasChanges(gitter, dir)
	if err != nil {
		return errors.Wrapf(err, "failed to detect if there were git changes in dir %s", dir)
	}
	if !changes {
		log.Logger().Infof("no changes detected so not creating a Pull Request on %s", termcolor.ColorInfo(gitURL))
		return nil
	}

	if branchName == "" {
		branchName, err = gitclient.CreateBranch(gitter, dir)
		if err != nil {
			return errors.Wrapf(err, "failed to create git branch in %s", dir)
		}
	}

	commitMessage := fmt.Sprintf("%s\n\n%s", commitTitle, commitBody)
	_, err = gitter.Command(dir, "commit", "-a", "-m", commitMessage, "--allow-empty")
	if err != nil {
		return errors.Wrapf(err, "failed to commit changes in dir %s", dir)
	}

	remote := "origin"
	_, err = gitter.Command(dir, "push", remote)
	if err != nil {
		return errors.Wrapf(err, "failed to push to remote %s from dir %s", remote, dir)
	}

	gitInfo, err := giturl.ParseGitURL(gitURL)
	if err != nil {
		return errors.Wrapf(err, "failed to parse git URL")
	}

	serverURL := gitInfo.HostURLWithoutUser()
	owner := gitInfo.Organisation

	scmClient := o.ScmClient
	if scmClient == nil {
		scmClient, _, err = o.CreateScmClient(serverURL, owner, kind)
		if err != nil {
			return errors.Wrapf(err, "failed to create SCM client for %s", gitURL)
		}
	}
	o.ScmClient = scmClient

	headPrefix := ""
	// if username is a fork then
	//	headPrefix = username + ":"

	head := headPrefix + branchName

	ctx := context.Background()
	pri := &scm.PullRequestInput{
		Title: commitTitle,
		Head:  head,
		Base:  "master",
		Body:  commitBody,
	}
	repoFullName := scm.Join(gitInfo.Organisation, gitInfo.Name)
	pr, _, err := scmClient.PullRequests.Create(ctx, repoFullName, pri)
	if err != nil {
		return errors.Wrapf(err, "failed to create PullRequest on %s", gitURL)
	}

	// the URL should not really end in .diff - fix in go-scm
	link := strings.TrimSuffix(pr.Link, ".diff")
	log.Logger().Infof("created Pull Request %s", termcolor.ColorInfo(link))
	return nil
}
