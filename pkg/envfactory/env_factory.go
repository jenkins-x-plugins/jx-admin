package envfactory

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/jenkins-x/go-scm/scm"
	"github.com/jenkins-x/jx-remote/pkg/authhelpers"
	"github.com/jenkins-x/jx-remote/pkg/common"
	"github.com/jenkins-x/jx-remote/pkg/githelpers"
	"github.com/jenkins-x/jx-remote/pkg/jxadapt"
	"github.com/jenkins-x/jx-remote/pkg/reqhelpers"
	"github.com/jenkins-x/jx/pkg/auth"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/jxfactory"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type EnvFactory struct {
	JXFactory            jxfactory.Factory
	Gitter               gits.Gitter
	AuthConfigService    auth.ConfigService
	RepoName             string
	GitURLOutFile        string
	OutDir               string
	IOFileHandles        *util.IOFileHandles
	ScmClient            *scm.Client
	BatchMode            bool
	UseGitHubOAuth       bool
	CreatedRepository    *CreateRepository
	CreatedScmRepository *scm.Repository
}

// AddFlags adds common CLI flags
func (o *EnvFactory) AddFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(&o.BatchMode, "batch-mode", "b", false, "Enables batch mode which avoids prompting for user input")
	cmd.Flags().BoolVarP(&o.UseGitHubOAuth, "oauth", "", false, "Enables the use of OAuth login to github.com to get a github access token")
	cmd.Flags().StringVarP(&o.RepoName, "repo", "", "", "the name of the development git repository to create")
	cmd.Flags().StringVarP(&o.GitURLOutFile, "out", "", "", "the name of the file to save with the created git URL inside")

}

// CreateDevEnvGitRepository creates the dev environment git repository from the given directory
func (o *EnvFactory) CreateDevEnvGitRepository(dir string, gitPublic bool) error {
	o.OutDir = dir
	requirements, fileName, err := config.LoadRequirementsConfig(dir)
	if err != nil {
		return errors.Wrapf(err, "failed to load requirements from %s", dir)
	}

	dev := reqhelpers.GetDevEnvironmentConfig(requirements)
	if dev == nil {
		return fmt.Errorf("the file %s does not contain a development environment", fileName)
	}

	cr := &CreateRepository{
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

	handles := jxadapt.ToIOHandles(o.IOFileHandles)
	err = cr.ConfirmValues(o.BatchMode, handles)
	if err != nil {
		return err
	}

	scmClient, token, err := o.CreateScmClient(cr.GitServer, cr.Owner, cr.GitKind)
	if err != nil {
		return errors.Wrapf(err, "failed to create SCM client for server %s", cr.GitServer)
	}
	o.ScmClient = scmClient

	user, _, err := scmClient.Users.Find(context.Background())
	if err != nil {
		return errors.Wrap(err, "failed to find the current SCM user")
	}
	cr.CurrentUsername = user.Login

	userAuth := &auth.UserAuth{
		Username: user.Login,
		ApiToken: token,
	}
	repo, err := cr.CreateRepository(scmClient)
	if err != nil {
		return err
	}
	o.CreatedScmRepository = repo
	err = o.PushToGit(repo.Clone, userAuth, dir)
	if err != nil {
		return errors.Wrap(err, "failed to push to the git repository")
	}
	err = o.PrintBootJobInstructions(requirements, repo.Link)
	if err != nil {
		return err
	}
	if o.GitURLOutFile != "" {
		err = ioutil.WriteFile(o.GitURLOutFile, []byte(repo.Link), util.DefaultFileWritePermissions)
		if err != nil {
			return errors.Wrapf(err, "failed to save Git URL to file %s", o.GitURLOutFile)
		}
	}
	return nil
}

// CreateScmClient creates a new scm client
func (o *EnvFactory) CreateScmClient(gitServer, owner, gitKind string) (*scm.Client, string, error) {
	af, err := authhelpers.NewAuthFacadeWithArgs(o.AuthConfigService, o.Gitter, o.IOFileHandles, o.BatchMode, o.UseGitHubOAuth)
	if err != nil {
		return nil, "", errors.Wrapf(err, "failed to create git auth facade")
	}
	scmClient, token, _, err := af.ScmClient(gitServer, owner, gitKind)
	if err != nil {
		return scmClient, token, errors.Wrapf(err, "failed to create SCM client for server %s", gitServer)
	}
	return scmClient, token, nil
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
	gitInfo, err := gits.ParseGitURL(link)
	if err != nil {
		return errors.Wrapf(err, "failed to parse git URL %s", link)
	}

	info := util.ColorInfo
	log.Logger().Info("\nto boot your cluster run the following commands:\n\n")

	log.Logger().Infof("%s", info(fmt.Sprintf("git clone %s", link)))
	log.Logger().Infof("%s", info(fmt.Sprintf("cd %s", gitInfo.Name)))
	log.Logger().Infof("%s", info(fmt.Sprintf("%s secrets edit", common.BinaryName)))
	log.Logger().Infof("%s", info(fmt.Sprintf("%s run", common.BinaryName)))
	log.Logger().Infof("\n\n")
	return nil
}

// PushToGit pushes to the git repository
func (o *EnvFactory) PushToGit(cloneURL string, userAuth *auth.UserAuth, dir string) error {
	forkPushURL, err := o.Gitter.CreateAuthenticatedURL(cloneURL, userAuth)
	if err != nil {
		return errors.Wrapf(err, "creating push URL for %s", cloneURL)
	}

	remoteBranch := "master"
	err = o.Gitter.Push(dir, forkPushURL, true, fmt.Sprintf("%s:%s", "HEAD", remoteBranch))
	if err != nil {
		return errors.Wrapf(err, "pushing merged branch %s", remoteBranch)
	}

	log.Logger().Infof("pushed code to the repository")
	return nil
}

// JXAdapter creates an adapter to the jx code
func (o *EnvFactory) JXAdapter() *jxadapt.JXAdapter {
	return jxadapt.NewJXAdapter(o.JXFactory, o.Gitter, o.BatchMode)
}

// CreatePullRequest crates a pull request if there are git changes
func (o *EnvFactory) CreatePullRequest(dir string, gitURL string, kind string, branchName string, commitTitle string, commitBody string) error {
	if gitURL == "" {
		log.Logger().Infof("no git URL specified so cannot create a Pull Request. Changes have been saved to %s", dir)
		return nil
	}

	gitter := o.Gitter
	changes, err := gitter.HasChanges(dir)
	if err != nil {
		return errors.Wrapf(err, "failed to detect if there were git changes in dir %s", dir)
	}
	if !changes {
		log.Logger().Infof("no changes detected so not creating a Pull Request on %s", util.ColorInfo(gitURL))
		return nil
	}

	if branchName == "" {
		branchName, err = githelpers.CreateBranch(gitter, dir)
		if err != nil {
			return errors.Wrapf(err, "failed to create git branch in %s", dir)
		}
	}

	commitMessage := fmt.Sprintf("%s\n\n%s", commitTitle, commitBody)
	err = gitter.AddCommit(dir, commitMessage)
	if err != nil {
		return errors.Wrapf(err, "failed to commit changes in dir %s", dir)
	}

	remote := "origin"
	err = gitter.Push(dir, remote, false)
	if err != nil {
		return errors.Wrapf(err, "failed to push to remote %s from dir %s", remote, dir)
	}

	gitInfo, err := gits.ParseGitURL(gitURL)
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
	log.Logger().Infof("created Pull Request %s", util.ColorInfo(link))
	return nil
}
