package operator

import (
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"strings"

	"github.com/jenkins-x/helmboot/pkg/apis/boot/v1alpha1"
	"github.com/jenkins-x/helmboot/pkg/bootconfig"
	"github.com/jenkins-x/helmboot/pkg/common"
	"github.com/jenkins-x/helmboot/pkg/helmer"
	"github.com/jenkins-x/helmboot/pkg/plugins/helmplugin"
	"github.com/jenkins-x/helmboot/pkg/reqhelpers"
	"github.com/jenkins-x/helmboot/pkg/rootcmd"
	"github.com/jenkins-x/helmboot/pkg/secretmgr"
	"github.com/jenkins-x/helmboot/pkg/secretmgr/factory"
	"github.com/jenkins-x/helmboot/pkg/versions"
	"github.com/jenkins-x/jx/pkg/cmd/boot"
	"github.com/jenkins-x/jx/pkg/cmd/clients"
	"github.com/jenkins-x/jx/pkg/cmd/helper"
	"github.com/jenkins-x/jx/pkg/cmd/opts"
	"github.com/jenkins-x/jx/pkg/cmd/templates"
	"github.com/jenkins-x/jx/pkg/config"
	"github.com/jenkins-x/jx/pkg/gits"
	"github.com/jenkins-x/jx/pkg/log"
	"github.com/jenkins-x/jx/pkg/util"
	"github.com/jenkins-x/jx/pkg/versionstream"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// Options contains the command line arguments for this command
type Options struct {
	boot.BootOptions
	KindResolver    factory.KindResolver
	Gitter          gits.Gitter
	ChartName       string
	ChartVersion    string
	ImageRepository string
	ImageTag        string
	GitUserName     string
	GitToken        string
	BatchMode       bool
	JobMode         bool
	NoTail          bool
}

var (
	cmdLong = templates.LongDesc(`
		Installs the git operator in a cluster

`)

	cmdExample = templates.Examples(`
		# installs the git operator
		%s operator 

		# installs the git operator with the given git clone URL
		%s run --url https://$GIT_USERNAME:$GIT_TOKEN@github.com/myorg/environment-mycluster-dev.git

`)
)

const (
	defaultChartName = "jx-labs/jx-git-operator"
)

// NewCmdRun creates the new command
func NewCmdOperator() (*cobra.Command, *Options) {
	options := &Options{}
	command := &cobra.Command{
		Use:     "operator",
		Short:   "installs the git operator in a cluster",
		Long:    cmdLong,
		Example: fmt.Sprintf(cmdExample, rootcmd.BinaryName, rootcmd.BinaryName),
		Run: func(command *cobra.Command, args []string) {
			common.SetLoggingLevel(command, args)
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	command.Flags().StringVarP(&options.Dir, "dir", "d", ".", "the directory to look for the Jenkins X Pipeline, requirements and charts")
	command.Flags().StringVarP(&options.GitURL, "git-url", "u", "", "override the Git clone URL for the JX Boot source to start from, ignoring the versions stream. Normally specified with git-ref as well")
	command.Flags().StringVarP(&options.GitUserName, "git-user", "", "", "specify the git user name to clone the development git repository. If not specified it is found from the secrets at $JX_SECRETS_YAML")
	command.Flags().StringVarP(&options.GitToken, "git-token", "", "", "specify the git token to clone the development git repository. If not specified it is found from the secrets at $JX_SECRETS_YAML")
	command.Flags().StringVarP(&options.GitRef, "git-ref", "", "master", "override the Git ref for the JX Boot source to start from, ignoring the versions stream. Normally specified with git-url as well")
	command.Flags().StringVarP(&options.ChartName, "chart", "c", defaultChartName, "the chart name to use to install the boot Job")
	command.Flags().StringVarP(&options.ChartVersion, "chart-version", "", "", "override the helm chart version used for the boot Job")
	command.Flags().StringVarP(&options.ImageRepository, "image-repository", "", "", "override the default docker image repository used by the boot job")
	command.Flags().StringVarP(&options.ImageTag, "image-tag", "", "", "override the default docker image tag used by the boot job")
	command.Flags().StringVarP(&options.VersionStreamURL, "versions-repo", "", common.DefaultVersionsURL, "the bootstrap URL for the versions repo. Once the boot config is cloned, the repo will be then read from the jx-requirements.yml")
	command.Flags().StringVarP(&options.VersionStreamRef, "versions-ref", "", common.DefaultVersionsRef, "the bootstrap ref for the versions repo. Once the boot config is cloned, the repo will be then read from the jx-requirements.yml")
	command.Flags().StringVarP(&options.HelmLogLevel, "helm-log", "v", "", "sets the helm logging level from 0 to 9. Passed into the helm CLI via the '-v' argument. Useful to diagnose helm related issues")
	command.Flags().StringVarP(&options.RequirementsFile, "requirements", "r", "", "requirements file which will overwrite the default requirements file")
	command.Flags().BoolVarP(&options.NoTail, "no-tail", "", false, "disables tailing the boot logs")
	command.Flags().BoolVarP(&options.JobMode, "job", "", false, "if running inside the cluster lets still default to creating the boot Job rather than running boot locally")

	defaultBatchMode := false
	if os.Getenv("JX_BATCH_MODE") == "true" {
		defaultBatchMode = true
	}
	command.PersistentFlags().BoolVarP(&options.BatchMode, "batch-mode", "b", defaultBatchMode, "Runs in batch mode without prompting for user input")

	return command, options
}

// Run installs the git operator chart
func (o *Options) Run() error {
	err := o.detectGitURL()
	if err != nil {
		return err
	}
	requirements, gitURL, err := reqhelpers.FindRequirementsAndGitURL(o.KindResolver.GetFactory(), o.GitURL, o.Git(), o.Dir)
	if err != nil {
		return err
	}
	if gitURL == "" {
		return util.MissingOption("url")
	}

	bootConfig, _, err := bootconfig.LoadBoot(o.Dir, false)
	if err != nil {
		return errors.Wrapf(err, "failed to load boot config in dir %s", o.Dir)
	}

	token, err := o.addUserPasswordForPrivateGitClone(bootConfig, false)
	if err != nil {
		return errors.Wrapf(err, "could not default the git user and token to clone the git URL")
	}

	clusterName := requirements.Cluster.ClusterName
	log.Logger().Infof("running the Boot Job for cluster %s with git URL %s", util.ColorInfo(clusterName), util.ColorInfo(util.SanitizeURL(gitURL)))

	helmBin, err := helmplugin.GetHelm3Binary()
	if err != nil {
		return err
	}

	log.Logger().Debug("deleting the old jx-boot chart ...")
	c := util.Command{
		Name: helmBin,
		Args: []string{"delete", "jx-boot"},
	}
	_, err = c.RunWithoutRetry()
	if err != nil {
		log.Logger().Debugf("failed to delete the old jx-boot chart: %s", err.Error())
	}

	// lets add helm repository for jx-labs
	h := helmplugin.NewHelmer(helmBin, o.Dir)
	_, err = helmer.AddHelmRepoIfMissing(h, helmer.LabsChartRepository, "jx-labs", "", "")
	if err != nil {
		return errors.Wrap(err, "failed to add Jenkins X Labs chart repository")
	}
	log.Logger().Debugf("updating helm repositories")
	err = h.UpdateRepo()
	if err != nil {
		log.Logger().Warnf("failed to update helm repositories: %s", err.Error())
	}

	if o.ChartVersion == "" {
		o.ChartVersion, err = o.findChartVersion(requirements)
		if err != nil {
			return err
		}
	}

	c = o.getCommandLine(helmBin, gitURL)

	// lets sanitize and format the command line so it looks nicer in the console output
	// TODO replace with c.CLI() when we switch to jx-helpers
	commandLine := fmt.Sprintf("%s %s", c.Name, strings.Join(c.Args, " "))
	if token == "" {
		u, err := url.Parse(gitURL)
		if err == nil && u.User != nil {
			token, _ = u.User.Password()
		}
	}
	if token != "" {
		commandLine = strings.ReplaceAll(commandLine, token, "****")
	}
	// lets split the command across lines
	commandLine = strings.ReplaceAll(commandLine, " --set", " \\\n    --set")

	log.Logger().Infof("running command:\n\n%s\n\n", util.ColorInfo(commandLine))

	_, err = c.RunWithoutRetry()
	if err != nil {
		return errors.Wrapf(err, "failed to run command %s", commandLine)
	}
	return nil
}

func (o *Options) getCommandLine(helmBin, gitURL string) util.Command {
	args := []string{"install"}

	if gitURL != "" {
		args = append(args, "--set", fmt.Sprintf("url=%s", gitURL))
	}
	if o.Version != "" {
		args = append(args, "--version", o.Version)
	}
	args = append(args, o.ReleaseName, "jx-git-operator")

	return util.Command{
		Name: helmBin,
		Args: args,
	}
}

// Git lazily create a gitter if its not specified
func (o *Options) Git() gits.Gitter {
	if o.Gitter == nil {
		o.Gitter = gits.NewGitCLI()
	}
	return o.Gitter
}

func (o *Options) findChartVersion(req *config.RequirementsConfig) (string, error) {
	if o.ChartName == "" || o.ChartName[0] == '.' || o.ChartName[0] == '/' || o.ChartName[0] == '\\' || strings.Count(o.ChartName, "/") > 1 {
		// relative chart folder so ignore version
		return "", nil
	}

	f := clients.NewFactory()
	co := opts.NewCommonOptionsWithTerm(f, os.Stdin, os.Stdout, os.Stderr)
	co.BatchMode = o.BatchMode

	u := req.VersionStream.URL
	ref := req.VersionStream.Ref
	version, err := versions.GetVersionNumber(versionstream.KindChart, o.ChartName, u, ref, o.Git(), co.GetIOFileHandles())
	if err != nil {
		return version, errors.Wrapf(err, "failed to find version of chart %s in version stream %s ref %s", o.ChartName, u, ref)
	}
	return version, nil
}

func (o *Options) addUserPasswordForPrivateGitClone(bootConfig *v1alpha1.Boot, inCluster bool) (string, error) {
	if o.GitUserName == "" && bootConfig != nil {
		o.GitUserName = bootConfig.Spec.PipelineBotUser
	}
	token := o.GitToken
	err := o.detectGitURL()
	if err != nil {
		return token, err
	}
	u, err := url.Parse(o.GitURL)
	if err != nil {
		return token, errors.Wrapf(err, "failed to parse git URL %s", o.GitURL)
	}

	// lets check if we've already got a user and password
	if u.User != nil {
		user := u.User
		pwd, f := user.Password()
		if user.Username() != "" && pwd != "" && f {
			return token, nil
		}
	}

	username := o.GitUserName

	fmt.Printf("username %s token %s\n", username, token)

	if username == "" || token == "" {
		if !inCluster || (bootConfig != nil && bootConfig.UsingExternalSecrets()) {
			if username == "" {
				return token, util.MissingOption("git-user")
			}
			return token, util.MissingOption("git-token")
		}
		yamlFile := os.Getenv("JX_SECRETS_YAML")
		if yamlFile == "" {
			if bootConfig != nil && bootConfig.UsingExternalSecrets() {
				return token, nil
			}
			return token, errors.Errorf("no $JX_SECRETS_YAML defined")
		}
		data, err := ioutil.ReadFile(yamlFile)
		if err != nil {
			return token, errors.Wrapf(err, "failed to load secrets YAML %s", yamlFile)
		}

		message := fmt.Sprintf("secrets YAML %s", yamlFile)
		username, token, err = secretmgr.PipelineUserTokenFromSecretsYAML(data, message)
		if err != nil {
			return token, err
		}
	}
	u.User = url.UserPassword(username, token)
	o.GitURL = u.String()
	o.KindResolver.GitURL = o.GitURL
	return token, nil
}

func (o *Options) defaultEnvVars() {
	if o.GitUserName == "" {
		o.GitUserName = os.Getenv("GIT_USERNAME")
	}
	if o.GitToken == "" {
		o.GitToken = os.Getenv("GIT_TOKEN")
	}
}

func (o *Options) detectGitURL() error {
	if o.GitURL == "" {
		// lets try load the git URL from the secret
		gitURL, err := o.KindResolver.LoadBootRunGitURLFromSecret()
		if err != nil {
			return errors.Wrapf(err, "failed to load the boot git URL from the Secret")
		}
		if gitURL == "" {
			log.Logger().Warnf("no git-url specified and no boot git URL Secret found")
		}
		o.GitURL = gitURL
	}
	o.KindResolver.GitURL = o.GitURL
	return nil
}
