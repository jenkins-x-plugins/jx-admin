package operator

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/jenkins-x/jx-admin/pkg/helmer"
	"github.com/jenkins-x/jx-admin/pkg/plugins/helmplugin"
	"github.com/jenkins-x/jx-admin/pkg/rootcmd"
	"github.com/jenkins-x/jx-api/pkg/config"
	"github.com/jenkins-x/jx-helpers/pkg/cobras/helper"
	"github.com/jenkins-x/jx-helpers/pkg/cobras/templates"
	"github.com/jenkins-x/jx-helpers/pkg/gitclient"
	"github.com/jenkins-x/jx-helpers/pkg/gitclient/gitconfig"
	"github.com/jenkins-x/jx-logging/pkg/log"
	"github.com/jenkins-x/jx/v2/pkg/gits"
	"github.com/jenkins-x/jx/v2/pkg/util"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// Options contains the command line arguments for this command
type Options struct {
	Dir          string
	GitURL       string
	GitUserName  string
	GitToken     string
	Namespace    string
	ReleaseName  string
	ChartName    string
	ChartVersion string
	DryRun       bool
	BatchMode    bool
}

var (
	cmdLong = templates.LongDesc(`
		Installs the git operator in a cluster

`)

	cmdExample = templates.Examples(`
* installs the git operator with the given git clone URL
` + bashExample("operator --url https://$GIT_USERNAME:$GIT_TOKEN@github.com/myorg/environment-mycluster-dev.git") + `
* installs the git operator from inside a git clone 
` + bashExample("operator --username mygituser --token mygittoken") + `
* installs the git operator and prompt the user for missing information
` + bashExample("operator") + `
`)
)

// bashExample returns markdown for a bash script expression
func bashExample(cli string) string {
	return fmt.Sprintf("\n```bash \n%s %s\n```\n", rootcmd.BinaryName, cli)
}

const (
	// DefaultOperatorNamespace the default namespace used to install the git operato
	DefaultOperatorNamespace = "jx"

	defaultChartName = "jx-labs/jx-git-operator"
)

// NewCmdRun creates the new command
func NewCmdOperator() (*cobra.Command, *Options) {
	options := &Options{}
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
	command.Flags().StringVarP(&options.GitUserName, "username", "", "", "specify the git user name to clone the environment git repository if there is no username in the git URL. If not specified defaults to $GIT_USERNAME")
	command.Flags().StringVarP(&options.GitToken, "token", "", "", "specify the git token to clone the environment git repository if there is no password in the git URL. If not specified defaults to $GIT_TOKEN")
	command.Flags().StringVarP(&options.Namespace, "namespace", "n", DefaultOperatorNamespace, "the namespace to install the git operator")

	defaultBatchMode := false
	if os.Getenv("JX_BATCH_MODE") == "true" {
		defaultBatchMode = true
	}
	command.PersistentFlags().BoolVarP(&options.BatchMode, "batch-mode", "b", defaultBatchMode, "Runs in batch mode without prompting for user input")

	options.AddFlags(command)

	return command, options
}

func (o *Options) AddFlags(command *cobra.Command) {
	command.Flags().StringVarP(&o.ReleaseName, "name", "", "jxgo", "the helm release name t ouse")
	command.Flags().StringVarP(&o.ChartName, "chart", "", defaultChartName, "the chart name to use to install the git operator")
	command.Flags().StringVarP(&o.ChartVersion, "chart-version", "", "", "override the helm chart version used for the git operator")
	command.Flags().BoolVarP(&o.DryRun, "dry-run", "", false, "if enabled just display the helm command that will run but don't actually do anything")
}

// Run installs the git operator chart
func (o *Options) Run() error {
	if o.GitUserName == "" {
		o.GitUserName = os.Getenv("GIT_USERNAME")
	}
	if o.GitToken == "" {
		o.GitToken = os.Getenv("GIT_TOKEN")
	}
	var err error
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
	_, err = helmer.AddHelmRepoIfMissing(h, helmer.LabsChartRepository, "jx-labs", "", "")
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
	token := ""
	if o.GitURL != "" {
		u, err := url.Parse(o.GitURL)
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

	if o.DryRun {
		return nil
	}
	_, err = c.RunWithoutRetry()
	if err != nil {
		return errors.Wrapf(err, "failed to run command %s", commandLine)
	}
	return nil
}

func (o *Options) getCommandLine(helmBin, gitURL string) util.Command {
	args := []string{"upgrade", "--install"}

	if gitURL != "" {
		args = append(args, "--set", fmt.Sprintf("url=%s", gitURL))
	}
	if o.ChartVersion != "" {
		args = append(args, "--version", o.ChartVersion)
	}
	if o.Namespace != "" {
		args = append(args, "--namespace", o.Namespace)
	}
	args = append(args, "--create-namespace", o.ReleaseName, o.ChartName)

	return util.Command{
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
	gitInfo, err := gits.ParseGitURL(gitURL)
	if err != nil {
		return gitURL, errors.Wrapf(err, "failed to parse git URL")
	}
	answer := gitInfo.HttpsURL()

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
			return answer, nil
		}
	}

	log.Logger().Infof("git clone URL is %s now adding the user/password so we can clone it inside kubernetes", util.ColorInfo(answer))

	// TODO if not batch mode ask the user for a username / token?
	if o.GitUserName == "" {
		return answer, util.MissingOption("username")
	}
	if o.GitToken == "" {
		return answer, util.MissingOption("token")
	}
	u.User = url.UserPassword(o.GitUserName, o.GitToken)
	answer = u.String()
	return answer, nil
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
