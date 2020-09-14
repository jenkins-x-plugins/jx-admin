package upgrade

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/jenkins-x/go-scm/scm"
	"github.com/jenkins-x/jx-admin/pkg/common"
	"github.com/jenkins-x/jx-admin/pkg/envfactory"
	"github.com/jenkins-x/jx-admin/pkg/reqhelpers"
	"github.com/jenkins-x/jx-admin/pkg/rootcmd"
	"github.com/jenkins-x/jx-admin/pkg/upgrader"
	v1 "github.com/jenkins-x/jx-api/pkg/apis/jenkins.io/v1"
	"github.com/jenkins-x/jx-api/pkg/client/clientset/versioned"
	"github.com/jenkins-x/jx-helpers/pkg/cobras/helper"
	"github.com/jenkins-x/jx-helpers/pkg/files"
	"github.com/jenkins-x/jx-helpers/pkg/gitclient"
	"github.com/jenkins-x/jx-helpers/pkg/gitclient/cli"
	"github.com/jenkins-x/jx-helpers/pkg/gitclient/giturl"
	"github.com/jenkins-x/jx-helpers/pkg/kube/jxclient"
	"github.com/jenkins-x/jx-helpers/pkg/termcolor"
	"github.com/jenkins-x/jx-kube-client/pkg/kubeclient"
	"github.com/jenkins-x/jx-logging/pkg/log"
	"sigs.k8s.io/yaml"

	"github.com/jenkins-x/jx-api/pkg/config"
	"github.com/jenkins-x/jx-helpers/pkg/cobras/templates"
	"github.com/jenkins-x/jx-helpers/pkg/versionstream"
	"github.com/jenkins-x/jx-helpers/pkg/versionstream/versionstreamrepo"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	upgradeLong = templates.LongDesc(`
		Upgrades your environment git repository to use the latest helm 3 / helmfile and version stream
`)

	upgradeExample = templates.Examples(`
		# upgrades your current environment's git repository
		%s upgrade
	`)
)

// Options the options for upgrading a cluster
type Options struct {
	envfactory.EnvFactory

	OverrideRequirements    config.RequirementsConfig
	IOFileHandles           *files.IOFileHandles
	Namespace               string
	GitCloneURL             string
	InitialGitURL           string
	Dir                     string
	UpgradeVersionStreamRef string
	branchName              string
	LatestRelease           bool
	UsePullRequest          bool
	NoCommit                bool
	GitCredentials          bool
	gitRepositoryExisted    bool // if we are modifying an existing git repository
}

// NewCmdUpgrade creates a command object for the command
func NewCmdUpgrade() (*cobra.Command, *Options) {
	o := &Options{}

	cmd := &cobra.Command{
		Use:     "upgrade",
		Short:   "Upgrades your environment git repository to use the latest helm 3 / helmfile and version stream",
		Long:    upgradeLong,
		Example: fmt.Sprintf(upgradeExample, rootcmd.BinaryName),
		Run: func(cmd *cobra.Command, args []string) {
			err := o.Run()
			helper.CheckErr(err)
		},
	}
	o.AddUpgradeOptions(cmd)
	return cmd, o
}

func (o *Options) AddUpgradeOptions(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&o.Dir, "dir", "d", "", "The directory used to create/clone the development git repository. If no directory is specified a temporary directory will be used")
	cmd.Flags().StringVarP(&o.UpgradeVersionStreamRef, "upgrade-version-stream-ref", "", config.DefaultVersionsRef, "a version stream ref to use to upgrade to")
	cmd.Flags().BoolVarP(&o.LatestRelease, "latest-release", "", false, "upgrade to latest release tag")
	cmd.Flags().BoolVarP(&o.GitCredentials, "git-credentials", "", false, "initialise the git credentials so that this step can be used inside a Job for use with private repositories")
	cmd.Flags().StringVarP(&o.GitCloneURL, "git-url", "g", "", "The git repository to clone to upgrade")
	cmd.Flags().StringVarP(&o.InitialGitURL, "initial-git-url", "", common.DefaultBootRepository, "The git URL to clone to fetch the initial set of files for a helm 3 / helmfile based git configuration if this command is not run inside a git clone or against a GitOps based cluster")
	cmd.Flags().BoolVarP(&o.UsePullRequest, "use-pr", "", false, "If enabled lets force the use of a Pull Request rather than creating a new git repository for the helm 3 based configuration")

	reqhelpers.AddGitRequirementsOptions(cmd, &o.OverrideRequirements)

	o.EnvFactory.AddFlags(cmd)
}

// Run implements the command
func (o *Options) Run() error {
	if o.Gitter == nil {
		o.Gitter = cli.NewCLIClient("", o.CommandRunner)
	}
	var err error
	o.JXClient, err = jxclient.LazyCreateJXClient(o.JXClient)
	if err != nil {
		return errors.Wrapf(err, "failed to ")
	}
	jxClient := o.JXClient
	ns := o.Namespace
	if ns == "" {
		ns, err = kubeclient.CurrentNamespace()
		if err != nil {
			return errors.Wrapf(err, "failed to get current namespace")
		}
	}
	envs, err := jxClient.JenkinsV1().Environments(ns).List(metav1.ListOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrapf(err, "failed to list Jenkins X Environments in namespace %s", ns)
	}

	// lets see if we are already on helm 3....
	dev, requirements, err := o.getDevAndRequirements(envs, ns)
	if err != nil {
		return errors.Wrapf(err, "failed to get the requirements from the dev environment in namespace %s", ns)
	}

	if dev != nil && requirements != nil && requirements.Helmfile {
		gitURL := dev.Spec.Source.URL
		if gitURL == "" {
			return errors.Errorf("cannot upgrade helm 3 installation as the dev Environment in namespace %s has no spec.source.url property", ns)
		}
		log.Logger().Infof("checking if we need to upgrade the installation for repository %s", termcolor.ColorInfo(gitURL))
		return o.UpgradeHelm3(dev)
	}

	log.Logger().Infof("attempting to migrate helm 2 to helm 3")
	return o.MigrateToHelm3(jxClient, ns, envs)
}

func (o *Options) getDevAndRequirements(envs *v1.EnvironmentList, ns string) (*v1.Environment, *config.RequirementsConfig, error) {
	for k := range envs.Items {
		env := envs.Items[k]
		if env.Name == "dev" {
			requirements, err := config.GetRequirementsConfigFromTeamSettings(&env.Spec.TeamSettings)
			devEnv := &env
			if err != nil {
				return devEnv, requirements, errors.Wrapf(err, "could not get requirements from Environment %s in namespace %s", env.Name, ns)
			}
			return devEnv, requirements, nil
		}
	}
	return nil, nil, nil
}

func (o *Options) UpgradeHelm3(devEnv *v1.Environment) error {
	gitter := o.Gitter
	envSource := devEnv.Spec.Source
	dir, err := o.gitCloneIfRequired(envSource)
	if err != nil {
		return errors.Wrap(err, "failed to git clone development environment source")
	}
	o.OutDir = dir

	requirements, requirementsFileName, err := config.LoadRequirementsConfig(dir, false)
	if err != nil {
		return errors.Wrapf(err, "failed to load requirements from dir %s", dir)
	}

	modified := false

	// now lets modify the requirements....
	reqsVersionStream := requirements.VersionStream
	versionsDir, err := o.cloneVersionStream(reqsVersionStream.URL, o.UpgradeVersionStreamRef, &devEnv.Spec.TeamSettings)
	if err != nil {
		return errors.Wrapf(err, "failed to clone the version stream %s", reqsVersionStream.URL)
	}

	upgradeVersionRef, err := o.upgradeAvailable(versionsDir, reqsVersionStream.Ref, o.UpgradeVersionStreamRef)
	if err != nil {
		return errors.Wrap(err, "failed to get check for available update")
	}
	if upgradeVersionRef == "" {
		log.Logger().Infof("No version upgrade found for version stream %s ref %s", termcolor.ColorInfo(reqsVersionStream.URL), termcolor.ColorInfo(reqsVersionStream.Ref))
		return nil
	}

	if requirements.VersionStream.Ref != upgradeVersionRef {
		log.Logger().Infof("Upgrading version stream ref to %s", termcolor.ColorInfo(upgradeVersionRef))
		requirements.VersionStream.Ref = upgradeVersionRef
		modified = true
	}

	// check if the build pack has changed in the version stream....
	buildPackURL := ""
	buildPackRef := ""
	if requirements.BuildPacks != nil && requirements.BuildPacks.BuildPackLibrary != nil {
		buildPackURL = requirements.BuildPacks.BuildPackLibrary.GitURL
		buildPackRef = requirements.BuildPacks.BuildPackLibrary.GitRef
	}
	if buildPackURL != "" {
		vs := &versionstream.VersionResolver{
			VersionsDir: versionsDir,
		}

		version, err := vs.StableVersionNumber(versionstream.KindGit, buildPackURL)
		if err != nil {
			log.Logger().Warnf("failed to find a version of repository %s in the version stream: %s", buildPackURL, err.Error())
		}

		if version != "" {
			if buildPackRef != version {
				log.Logger().Infof("Upgrading build pack repository %s to version %s", termcolor.ColorInfo(buildPackURL), termcolor.ColorInfo(version))
				requirements.BuildPacks.BuildPackLibrary.GitRef = version
				modified = true
			} else {
				log.Logger().Infof("Build pack repository %s already on version %s", termcolor.ColorInfo(buildPackURL), termcolor.ColorInfo(version))
			}

			/*
				TODO

				// lets check if we need to upgrade the project config
				projectConfig, projectConfigFile, err := config.LoadProjectConfig(dir)
				if err != nil {
					return errors.Wrapf(err, "failed to load project config from dir %s", dir)
				}
				if projectConfig.BuildPackGitURef != version {
					projectConfig.BuildPackGitURef = version
					err = projectConfig.SaveConfig(projectConfigFile)
					if err != nil {
						return errors.Wrapf(err, "failed to save project config file %s", projectConfigFile)
					}
					modified = true
				}

			*/
		}
	}

	if !modified {
		log.Logger().Infof("no changes were made to repository %s so no Pull Request created", envSource.URL)
		return nil
	}

	o.branchName, err = gitclient.CreateBranch(o.Gitter, dir)
	if err != nil {
		return errors.Wrapf(err, "failed to create git branch in %s", dir)
	}

	err = requirements.SaveConfig(requirementsFileName)
	if err != nil {
		return errors.Wrapf(err, "failed to save the requirements to dir %s", requirementsFileName)
	}

	_, err = gitter.Command(dir, "add", "*")
	if err != nil {
		return errors.Wrapf(err, "failed to add to git in dir %s", dir)
	}
	changes, err := gitclient.HasChanges(gitter, dir)
	if err != nil {
		return errors.Wrapf(err, "failed to check if there are git changes in dir %s", dir)
	}
	if !changes {
		log.Logger().Infof("no changes were made to repository %s so no Pull Request created", envSource.URL)
		return nil
	}

	err = gitclient.CommitIfChanges(gitter, dir, "fix: upgrade version stream")
	if err != nil {
		return errors.Wrapf(err, "failed to git commit changes")
	}

	kind := reqhelpers.GitKind(envSource, requirements)
	err = o.createPullRequest(dir, kind, "requirements: upgrade versions")
	if err != nil {
		return errors.Wrapf(err, "failed to create Pull Request for version upgrades in dir %s", dir)
	}
	return nil
}

func (o *Options) upgradeAvailable(versionsDir, versionStreamRef, upgradeRef string) (string, error) {
	gitter := o.Gitter
	var err error
	if o.LatestRelease {
		_, upgradeRef, err = gitclient.GetCommitPointedToByLatestTag(gitter, versionsDir)
		if err != nil {
			return "", errors.Wrapf(err, "failed to get latest tag at %s", o.Dir)
		}
	} else {
		upgradeRef, err = gitter.Command(versionsDir, "rev-list", "-n", "1", upgradeRef)
		if err != nil {
			return "", errors.Wrapf(err, "failed to get commit pointed to by %s", upgradeRef)
		}
	}

	if versionStreamRef == upgradeRef {
		log.Logger().Infof(termcolor.ColorInfo("No version stream upgrade available"))
		return "", nil
	}
	log.Logger().Infof(termcolor.ColorInfo("Version stream upgrade available"))
	return upgradeRef, nil
}

func (o *Options) cloneVersionStream(versionStreamURL, upgradeRef string, settings *v1.TeamSettings) (string, error) {
	gitter := o.Gitter
	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		return "", errors.Wrap(err, "failed to create temporary dir for version stream")
	}
	versionsDir, _, err := versionstreamrepo.CloneJXVersionsRepoToDir(tempDir, versionStreamURL, upgradeRef, settings, gitter, o.BatchMode, false, files.GetIOFileHandles(o.IOFileHandles))
	if err != nil {
		return "", errors.Wrapf(err, "failed to clone versions repo %s", versionStreamURL)
	}
	return versionsDir, nil
}

func (o *Options) MigrateToHelm3(jxClient versioned.Interface, ns string, envs *v1.EnvironmentList) error {
	u := &upgrader.HelmfileUpgrader{
		Environments:         envs.Items,
		OverrideRequirements: &o.OverrideRequirements,
	}

	req, err := u.ExportRequirements()
	if err != nil {
		return errors.Wrapf(err, "failed to generate the JX Requirements from the cluster")
	}

	dir, err := o.gitCloneIfRequired(u.DevSource)
	if err != nil {
		return err
	}
	o.OutDir = dir

	if o.gitRepositoryExisted {
		o.branchName, err = gitclient.CreateBranch(o.Gitter, dir)
		if err != nil {
			return errors.Wrapf(err, "failed to create git branch in %s", dir)
		}
	}

	reqFile := filepath.Join(dir, config.RequirementsConfigFileName)
	err = req.SaveConfig(reqFile)
	if err != nil {
		return errors.Wrapf(err, "failed to save migrated requirements file %s", reqFile)
	}

	err = o.replacePipeline(dir)
	if err != nil {
		return err
	}

	err = o.removeGeneratedRequirementsValuesFile(dir)
	if err != nil {
		return err
	}

	log.Logger().Infof("generated the latest cluster requirements configuration to %s", termcolor.ColorInfo(reqFile))

	err = o.addAndRemoveFiles(dir, jxClient, ns)
	if err != nil {
		return err
	}

	log.Logger().Infof("generated the boot configuration from the current cluster into the directory: %s", termcolor.ColorInfo(dir))

	// now lets add the generated files to git
	_, err = o.Gitter.Command(dir, "add", "*")
	if err != nil {
		return errors.Wrapf(err, "failed to add files to git")
	}

	if o.gitRepositoryExisted && o.UsePullRequest {
		o.OutDir = dir
		if !o.NoCommit {
			err = gitclient.CommitIfChanges(o.Gitter, dir, "fix: helmboot upgrader\n\nmigrating resources across to the latest Jenkins X GitOps source code")
			if err != nil {
				return errors.Wrapf(err, "failed to git commit changes")
			}

			if o.GitCloneURL != "" {
				kind := reqhelpers.GitKind(u.DevSource, u.Requirements)
				return o.createPullRequest(dir, kind, "fix: upgrade to helmfile + helm 3")
			}

			return o.EnvFactory.PrintBootJobInstructions(req, o.GitCloneURL)
		}
		return nil
	}
	return o.EnvFactory.CreateDevEnvGitRepository(dir, req.Cluster.EnvironmentGitPublic)
}

func (o *Options) removeGeneratedRequirementsValuesFile(dir string) error {
	// lets remove the extra yaml file used during the boot process (we should disable this via a flag via changing the jx code)
	requirementsValuesFile := filepath.Join(dir, config.RequirementsValuesFileName)
	exists, err := files.FileExists(requirementsValuesFile)
	if err != nil {
		return errors.Wrapf(err, "failed to check requirements values file exists %s", requirementsValuesFile)
	}
	if exists {
		err = os.Remove(requirementsValuesFile)
		if err != nil {
			return errors.Wrapf(err, "failed to remove file %s", requirementsValuesFile)
		}
	}
	return nil
}

func (o *Options) addAndRemoveFiles(dir string, jxClient versioned.Interface, ns string) error {
	err := o.removeOldDirs(dir)
	if err != nil {
		return err
	}

	err = o.addMissingFiles(dir)
	if err != nil {
		return err
	}

	srOutDir := filepath.Join(dir, "repositories", "templates")
	err = os.MkdirAll(srOutDir, files.DefaultDirWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to create the SourceRepository output directory: %s", srOutDir)
	}

	err = o.writeAdditionalHelmTemplateFiles(jxClient, ns, srOutDir)
	if err != nil {
		return errors.Wrapf(err, "failed to write additional helm template files")
	}

	err = o.writeNonHelmManagedResources(jxClient, ns, dir)
	if err != nil {
		return errors.Wrapf(err, "failed to write migration files")
	}
	return nil
}

// addMissingFiles if the current dir is an old helm 2 style repository
// lets copy across any new directories/files from the template git repository
func (o *Options) addMissingFiles(dir string) error {
	templateDir := ""
	lazyCloneTemplates := func() error {
		if templateDir == "" {
			dir, err := gitclient.CloneToDir(o.Gitter, o.InitialGitURL, "")
			if err != nil {
				return errors.Wrapf(err, "failed to git clone %s", o.InitialGitURL)
			}
			templateDir = dir
		}
		return nil
	}

	fileNames := []string{"jx-apps.yml"}

	for _, name := range fileNames {
		f := filepath.Join(dir, name)
		exists, err := files.FileExists(f)
		if err != nil {
			return errors.Wrapf(err, "failed to check file exists %s", f)
		}
		if !exists {
			err = lazyCloneTemplates()
			if err != nil {
				return err
			}
			err = files.CopyFile(filepath.Join(templateDir, name), f)
			if err != nil {
				return errors.Wrapf(err, "failed to copy missing file %s", f)
			}
		}
	}
	return nil
}

// removeOldDirs lets remove any old files/directories from the helm 2.x style git repository
func (o *Options) removeOldDirs(dir string) error {
	oldDirs := []string{"env", "systems", "kubeProviders", "prowConfig"}
	for _, od := range oldDirs {
		oldDir := filepath.Join(dir, od)
		exists, err := files.DirExists(oldDir)
		if err != nil {
			return errors.Wrapf(err, "failed to check dir exists %s", oldDir)
		}
		if exists {
			err = os.RemoveAll(oldDir)
			if err != nil {
				return errors.Wrapf(err, "failed to remove dir %s", oldDir)
			}
			log.Logger().Infof("removed old folder %s", oldDir)
		}
	}
	return nil
}

// writeAdditionalHelmTemplateFiles lets store to git any extra resources managed outside of the regular boot charts
func (o *Options) writeAdditionalHelmTemplateFiles(jxClient versioned.Interface, ns, outDir string) error {
	// lets write the SourceRepository resources to the repositories folder...
	srList, err := jxClient.JenkinsV1().SourceRepositories(ns).List(metav1.ListOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrapf(err, "failed to query the SourceRepository resources in namespace %s", ns)
	}
	_, err = upgrader.WriteSourceRepositoriesToGitFolder(outDir, srList)
	if err != nil {
		return errors.Wrapf(err, "failed to write SourceRepository resources to %s", outDir)
	}
	return nil
}

// writeAdditionalHelmTemplateFiles lets store to git any extra resources managed outside of the regular boot charts
func (o *Options) writeNonHelmManagedResources(jxClient versioned.Interface, ns, dir string) error {
	paList, err := jxClient.JenkinsV1().PipelineActivities(ns).List(metav1.ListOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrapf(err, "failed to query the PipelineActivity resources in namespace %s", ns)
	}
	if len(paList.Items) == 0 {
		return nil
	}

	// lets clear out unnecessary metadata
	paList.ListMeta = metav1.ListMeta{}
	for i := range paList.Items {
		pa := &paList.Items[i]
		pa.ObjectMeta = upgrader.EmptyObjectMeta(&pa.ObjectMeta)
	}

	data, err := yaml.Marshal(paList)
	if err != nil {
		return errors.Wrapf(err, "failed to marshal PipelineActivity resources to YAML")
	}

	fileName := filepath.Join(dir, common.PipelineActivitiesYAMLFile)
	err = ioutil.WriteFile(fileName, data, files.DefaultFileWritePermissions)
	if err != nil {
		return errors.Wrapf(err, "failed to write file %s", fileName)
	}
	log.Logger().Infof("wrote migration resources file: %s", termcolor.ColorInfo(common.PipelineActivitiesYAMLFile))
	return nil
}

// gitCloneIfRequired if the specified directory is already a git clone then lets just use it
// otherwise lets make a temporary directory and clone the git repository specified
// or if there is none make a new one
func (o *Options) gitCloneIfRequired(devSource v1.EnvironmentRepository) (string, error) {
	if o.GitCredentials {
		err := o.getGitCredentials()
		if err != nil {
			return "", errors.Wrap(err, "failed to get git credentials")
		}
	}
	o.gitRepositoryExisted = true
	gitURL := o.GitCloneURL
	if gitURL == "" {
		gitURL = devSource.URL
		o.GitCloneURL = gitURL
		if gitURL == "" {
			gitURL = o.InitialGitURL
			o.gitRepositoryExisted = false
		}
	}
	var err error
	dir := o.Dir
	if dir == "" {
		dir, err = ioutil.TempDir("", "helmboot-")
		if err != nil {
			return "", errors.Wrap(err, "failed to create temporary directory")
		}
	} else {
		// if you specify and it has a git clone inside lets just use it rather than cloning
		// as you may be inside a fork or something
		d, _, err := gitclient.FindGitConfigDir(dir)
		if err != nil {
			return "", errors.Wrapf(err, "there was a problem obtaining the git config dir of directory %s", dir)
		}
		if d != "" {
			o.gitRepositoryExisted = true
			return dir, nil
		}
	}

	log.Logger().Debugf("cloning %s to directory %s", termcolor.ColorInfo(gitURL), termcolor.ColorInfo(dir))

	dir, err = gitclient.CloneToDir(o.Gitter, gitURL, dir)
	if err != nil {
		return "", errors.Wrapf(err, "failed to clone repository %s to directory: %s", gitURL, dir)
	}
	return dir, nil
}

// replacePipeline if the `jenkins-x.yml` file is missing or does use the helm 3 / helmfile style configuration
// lets replace with the new pipeline file
func (o *Options) replacePipeline(dir string) error {
	/* TODO
	projectConfig, fileName, err := config.LoadProjectConfig(dir)
	if err != nil {
		return errors.Wrap(err, "failed to load Jenkins X Pipeline")
	}
	if projectConfig.BuildPack == common.HelmfileBuildPackName {
		return nil
	}
	projectConfig = &config.ProjectConfig{}
	projectConfig.BuildPack = common.HelmfileBuildPackName

	err = projectConfig.SaveConfig(fileName)
	if err != nil {
		return errors.Wrap(err, "failed to save Jenkins X Pipeline")
	}
	*/
	return nil
}

func (o *Options) createPullRequest(dir, kind, title string) error {
	remote := "origin"

	log.Logger().Infof("pushing commits to ")
	_, err := o.Gitter.Command(dir, "push", remote, o.branchName)
	if err != nil {
		return errors.Wrapf(err, "failed to push to remote %s from dir %s", remote, dir)
	}

	gitURL := o.GitCloneURL
	gitInfo, err := giturl.ParseGitURL(gitURL)
	if err != nil {
		return errors.Wrapf(err, "failed to parse git URL")
	}

	serverURL := gitInfo.HostURLWithoutUser()
	owner := gitInfo.Organisation
	scmClient, _, err := o.EnvFactory.CreateScmClient(serverURL, owner, kind)
	if err != nil {
		return errors.Wrapf(err, "failed to create SCM client for %s", gitURL)
	}
	o.ScmClient = scmClient

	headPrefix := ""
	// if username is a fork then
	//	headPrefix = username + ":"

	head := headPrefix + o.branchName

	ctx := context.Background()
	pri := &scm.PullRequestInput{
		Title: title,
		Head:  head,
		Base:  "master",
		Body:  "",
	}
	repoFullName := scm.Join(gitInfo.Organisation, gitInfo.Name)
	pr, _, err := scmClient.PullRequests.Create(ctx, repoFullName, pri)
	if err != nil {
		return errors.Wrapf(err, "failed to create PullRequest on %s", gitURL)
	}

	// the URL should not really end in .diff - fix in go-scm
	link := strings.TrimSuffix(pr.Link, ".diff")
	log.Logger().Infof("created Pull Request %s in dir %s", termcolor.ColorInfo(link), dir)
	return nil
}

func (o *Options) getGitCredentials() error {
	/*  TODO

	log.Logger().Infof("setting up git credentials")

	a := jxadapt.NewJXAdapter(o.JXFactory, o.Gitter, o.BatchMode)
	co := a.NewCommonOptions()

	err := co.InitGitConfigAndUser()
	if err != nil {
		return errors.Wrap(err, "failed to setup git user and credentials store")
	}

	so := &credentials.StepGitCredentialsOptions{}
	so.CommonOptions = co
	err = so.Run()
	if err != nil {
		return errors.Wrap(err, "failed to setup git credentials")
	}

	*/
	return nil
}
