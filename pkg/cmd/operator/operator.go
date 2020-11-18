package operator

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/jenkins-x/jx-admin/pkg/cmd/joblog"
	"github.com/jenkins-x/jx-admin/pkg/common"
	"github.com/jenkins-x/jx-admin/pkg/plugins/helmplugin"
	"github.com/jenkins-x/jx-admin/pkg/rootcmd"
	"github.com/jenkins-x/jx-api/v3/pkg/config"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cmdrunner"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cobras/helper"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cobras/templates"
	"github.com/jenkins-x/jx-helpers/v3/pkg/gitclient"
	"github.com/jenkins-x/jx-helpers/v3/pkg/gitclient/gitconfig"
	"github.com/jenkins-x/jx-helpers/v3/pkg/gitclient/giturl"
	"github.com/jenkins-x/jx-helpers/v3/pkg/helmer"
	"github.com/jenkins-x/jx-helpers/v3/pkg/input/survey"
	"github.com/jenkins-x/jx-helpers/v3/pkg/kube"
	"github.com/jenkins-x/jx-helpers/v3/pkg/options"
	"github.com/jenkins-x/jx-helpers/v3/pkg/termcolor"
	"github.com/jenkins-x/jx-kube-client/v3/pkg/kubeclient"
	"github.com/jenkins-x/jx-logging/v3/pkg/log"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
)

// Options contains the command line arguments for this command
type Options struct {
	Dir               string
	GitURL            string
	GitUserName       string
	GitToken          string
	Namespace         string
	ReleaseName       string
	ChartName         string
	ChartVersion      string
	DryRun            bool
	NoSwitchNamespace bool
	NoLog             bool
	BatchMode         bool
	JobLogOptions     joblog.Options
}

var (
	cmdLong = templates.LongDesc(`
		Installs the git operator in a cluster

`)

	cmdExample = templates.Examples(`
* installs the git operator from inside a git clone and prompt for the user/token if required 
` + bashExample("operator") + `
* installs the git operator from inside a git clone specifying the user/token 
` + bashExample("operator --username mygituser --token mygittoken") + `
* installs the git operator with the given git clone URL
` + bashExample("operator --url https://github.com/myorg/environment-mycluster-dev.git --username myuser --token myuser") + `
* display what helm command will install the git operator
` + bashExample("operator --dry-run") + `
`)
)

// bashExample returns markdown for a bash script expression
func bashExample(cli string) string {
	return fmt.Sprintf("\n```bash \n%s %s\n```\n", rootcmd.BinaryName, cli)
}

const (
	defaultChartName = "jx3/jx-git-operator"
)

// NewCmdRun creates the new command
func NewCmdOperator() (*cobra.Command, *Options) {
	options := &Options{}

	// add defaults
	_, jo := joblog.NewCmdJobLog()
	options.JobLogOptions = *jo

	command := &cobra.Command{
		Use:     "operator",
		Short:   "installs the git operator in a cluster",
		Aliases: []string{"boot"},
		Long:    cmdLong,
		Example: cmdExample,
		Run: func(command *cobra.Command, args []string) {
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	command.Flags().StringVarP(&options.Dir, "dir", "d", ".", "the directory to discover the git URL if no url option is specified")
	command.Flags().StringVarP(&options.GitURL, "url", "u", "", "the git URL for the environment to boot using the operator. This is optional - the git operator Secret can be created later")
	command.Flags().StringVarP(&options.GitUserName, "username", "", "", "specify the git user name the operator will use to clone the environment git repository if there is no username in the git URL. If not specified defaults to $GIT_USERNAME")
	command.Flags().StringVarP(&options.GitToken, "token", "", "", "specify the git token the operator will use to clone the environment git repository if there is no password in the git URL. If not specified defaults to $GIT_TOKEN")
	command.Flags().StringVarP(&options.Namespace, "namespace", "n", common.DefaultOperatorNamespace, "the namespace to install the git operator")
	command.Flags().StringVarP(&options.ReleaseName, "name", "", "jxgo", "the helm release name t ouse")
	command.Flags().StringVarP(&options.ChartName, "chart", "", defaultChartName, "the chart name to use to install the git operator")
	command.Flags().StringVarP(&options.ChartVersion, "chart-version", "", "", "override the helm chart version used for the git operator")

	command.Flags().BoolVarP(&options.NoLog, "no-log", "", false, "to disable viewing the logs of the boot Job pods")
	command.Flags().BoolVarP(&options.NoSwitchNamespace, "no-switch-namespace", "", false, "to disable switching to the installation namespace after installing the operator")
	command.Flags().BoolVarP(&options.DryRun, "dry-run", "", false, "if enabled just display the helm command that will run but don't actually do anything")

	command.Flags().DurationVarP(&options.JobLogOptions.Duration, "max-log-duration", "", time.Minute*30, "how long to wait for a boot Job pod to be ready to view its log")

	defaultBatchMode := false
	if os.Getenv("JX_BATCH_MODE") == "true" {
		defaultBatchMode = true
	}

	//Todo: Do we really need a persistent flag here?
	command.PersistentFlags().BoolVarP(&options.BatchMode, "batch-mode", "b", defaultBatchMode, "Runs in batch mode without prompting for user input")

	return command, options
}

// Run installs the git operator chart
func (o *Options) Run() error {
	var err error
	if o.GitUserName == "" {
		o.GitUserName = os.Getenv("GIT_USERNAME")
	}
	if o.GitToken == "" {
		o.GitToken = os.Getenv("GIT_TOKEN")
	}
	if o.GitURL == "" {
		o.GitURL, err = findGitURLFromDir(o.Dir)
		if err != nil {
			return errors.Wrapf(err, "failed to detect the git URL from the directory %s", o.Dir)
		}
	}
	if o.GitURL != "" {
		o.GitURL, err = o.ensureValidGitURL(o.GitURL)
		if err != nil {
			return errors.Wrapf(err, "failed to ensure the git URL is valid")
		}
	}
	helmBin, err := helmplugin.GetHelm3Binary()
	if err != nil {
		return err
	}

	// lets add helm repository for jx-labs
	h := helmplugin.NewHelmer(helmBin, o.Dir)
	_, err = helmer.AddHelmRepoIfMissing(h, helmer.JX3ChartRepository, "jx3", "", "")
	if err != nil {
		return errors.Wrap(err, "failed to add Jenkins X Labs chart repository")
	}
	log.Logger().Debugf("updating helm repositories")
	err = h.UpdateRepo()
	if err != nil {
		log.Logger().Warnf("failed to update helm repositories: %s", err.Error())
	}

	c := o.getCommandLine(helmBin, o.GitURL)

	// lets sanitize and format the command line so it looks nicer in the console output
	// TODO replace with c.CLI() when we switch to jx-helpers
	commandLine := fmt.Sprintf("%s %s", c.Name, strings.Join(c.Args, " "))
	token := o.GitToken
	if o.GitURL != "" && token == "" {
		u, err := url.Parse(o.GitURL)
		if err == nil && u.User != nil {
			token, _ = u.User.Password()
		}
	}
	if token != "" && !o.DryRun {
		commandLine = strings.ReplaceAll(commandLine, token, "****")
	}

	// lets split the command across lines
	commandLine = strings.ReplaceAll(commandLine, " --set", " \\\n    --set")

	if o.DryRun {
		log.Logger().Infof("\nTo install the git operator run this command:\n\n%s\n\n", termcolor.ColorInfo(commandLine))
		return nil
	}

	log.Logger().Infof("running command:\n\n%s\n\n", termcolor.ColorInfo(commandLine))

	_, err = c.RunWithoutRetry()
	if err != nil {
		return errors.Wrapf(err, "failed to run command %s", commandLine)
	}

	err = o.switchNamespace(o.Namespace)
	if err != nil {
		return errors.Wrapf(err, "failed to switch the kubernetes namespace")
	}

	if o.NoLog {
		return nil
	}
	o.JobLogOptions.WaitMode = true
	err = o.JobLogOptions.Run()
	if err != nil {
		return errors.Wrapf(err, "failed to tail the Jenkins X boot Job pods")
	}
	return nil
}

func (o *Options) getCommandLine(helmBin, gitURL string) cmdrunner.Command {
	username := o.GitUserName
	password := o.GitToken

	args := []string{"upgrade", "--install"}

	if gitURL != "" {
		args = append(args, "--set", fmt.Sprintf("url=%s", gitURL))
	}
	if username != "" {
		args = append(args, "--set", fmt.Sprintf("username=%s", username))
	}
	if password != "" {
		args = append(args, "--set", fmt.Sprintf("password=%s", password))
	}
	if o.ChartVersion != "" {
		args = append(args, "--version", o.ChartVersion)
	}
	if o.Namespace != "" {
		args = append(args, "--namespace", o.Namespace)
	}
	args = append(args, "--create-namespace", o.ReleaseName, o.ChartName)

	return cmdrunner.Command{
		Name: helmBin,
		Args: args,
	}
}

//nolint
func (o *Options) findChartVersion(_ *config.RequirementsConfig) (string, error) {
	/*
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
	*/
	return "", nil
}

func (o *Options) ensureValidGitURL(gitURL string) (string, error) {
	answer := gitURL

	u, err := url.Parse(answer)
	if err != nil {
		return answer, errors.Wrapf(err, "failed to parse git URL %s", answer)
	}

	// lets check if we've already got a user and password
	if u.User != nil {
		user := u.User
		pwd, f := user.Password()
		username := user.Username()
		if username != "" && pwd != "" && f {
			if o.GitUserName == "" {
				o.GitUserName = username
			}
			if o.GitToken == "" {
				o.GitToken = pwd
			}
		}

		// lets remove the user/pwd from the URL
		u.User = nil
		answer = u.String()
	}

	log.Logger().Infof("git clone URL is %s", termcolor.ColorInfo(answer))
	log.Logger().Infof("now verifying we have a valid git username and token so that we can clone the git repository inside kubernetes...")

	if o.GitUserName == "" {
		if !o.BatchMode {
			i := survey.NewInput()
			o.GitUserName, err = i.PickValue("Enter Bot Git username the Kubernetes operator will use to clone the environment git repository", "", true, "The Kubernetes Git Operator synchronises the environment git repository into the cluster")
			if err != nil {
				return answer, errors.Wrap(err, "failed to get git username")
			}
		} else {
			return answer, options.MissingOption("username")
		}

	}
	if o.GitToken == "" {
		requirements, _, err := config.LoadRequirementsConfig(o.Dir, false)
		if err != nil {
			return answer, errors.Wrapf(err, "cannot load requirements file in dir %s so cannot determine git kind", o.Dir)
		}
		giturl.PrintCreateRepositoryGenerateAccessToken(requirements.Cluster.GitKind, requirements.Cluster.GitServer, o.GitUserName, os.Stdout)

		if !o.BatchMode {
			i := survey.NewInput()
			o.GitToken, err = i.PickPassword("Enter Bot Git token the Kubernetes operator will use to clone the environment git repository", "The Kubernetes Git Operator synchronises the environment git repository into the cluster, the token only requires read repository permissions and the token is stored in a Kubernetes secrets the job access")
			if err != nil {
				return answer, errors.Wrap(err, "failed to get git password")
			}
		} else {
			return answer, options.MissingOption("token")
		}
	}
	log.Logger().Infof("git username is %s for URL %s and we have a valid password", termcolor.ColorInfo(o.GitUserName), termcolor.ColorInfo(answer))
	return answer, nil
}

func (o *Options) switchNamespace(ns string) error {
	if o.NoSwitchNamespace {
		log.Logger().Infof("disabled switching namespace. Please make sure you are in the %s namespace when you try to create or import a project", ns)
		return nil
	}
	cfg, pathOptions, err := kubeclient.LoadConfig()
	if err != nil {
		return errors.Wrap(err, "loading Kubernetes configuration")
	}
	ctx := kube.CurrentContext(cfg)
	if ctx == nil {
		log.Logger().Warnf("there is no context defined in your Kubernetes configuration so cannot change to namepace %s - we may be inside a test case or pod?", ns)
		return nil
	}

	if ctx.Namespace == ns {
		return nil
	}
	ctx.Namespace = ns
	err = clientcmd.ModifyConfig(pathOptions, *cfg, false)
	if err != nil {
		return errors.Wrapf(err, "failed to update the kube config to namepace %s", ns)
	}
	log.Logger().Infof("switched to namespace %s so that you can start to create or import projects into Jenkins X: https://jenkins-x.io/docs/v3/create-project/", termcolor.ColorInfo(ns))
	return nil
}

func findGitURLFromDir(dir string) (string, error) {
	_, gitConfDir, err := gitclient.FindGitConfigDir(dir)
	if err != nil {
		return "", errors.Wrapf(err, "there was a problem obtaining the git config dir of directory %s", dir)
	}
	if gitConfDir == "" {
		return "", nil
	}
	return gitconfig.DiscoverUpstreamGitURL(gitConfDir)
}
