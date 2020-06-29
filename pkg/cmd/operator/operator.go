package operator

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/jenkins-x/helmboot/pkg/common"
	"github.com/jenkins-x/helmboot/pkg/helmer"
	"github.com/jenkins-x/helmboot/pkg/plugins/helmplugin"
	"github.com/jenkins-x/helmboot/pkg/rootcmd"
	"github.com/jenkins-x/helmboot/pkg/versions"
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
	Gitter       gits.Gitter
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
	defaultChartName = "jx-labs/jx-git-operator"
)

// NewCmdRun creates the new command
func NewCmdOperator() (*cobra.Command, *Options) {
	options := &Options{}
	command := &cobra.Command{
		Use:     "operator",
		Short:   "installs the git operator in a cluster",
		Long:    cmdLong,
		Example: fmt.Sprintf(cmdExample, rootcmd.BinaryName, rootcmd.BinaryName, rootcmd.BinaryName),
		Run: func(command *cobra.Command, args []string) {
			common.SetLoggingLevel(command, args)
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	command.Flags().StringVarP(&options.Dir, "dir", "d", ".", "the directory to discover the git URL if no url option is specified")
	command.Flags().StringVarP(&options.GitURL, "url", "u", "", "the git URL for the environment to boot using the operator. This is optional - the git operator Secret can be created later")
	command.Flags().StringVarP(&options.GitUserName, "username", "", "", "specify the git user name to clone the environment git repository if there is no username in the git URL. If not specified defaults to $GIT_USERNAME")
	command.Flags().StringVarP(&options.GitToken, "token", "", "", "specify the git token to clone the environment git repository if there is no password in the git URL. If not specified defaults to $GIT_TOKEN")
	command.Flags().StringVarP(&options.Namespace, "namespace", "n", "jx-git-operator", "the namespace to install the git operator")
	command.Flags().StringVarP(&options.ReleaseName, "name", "", "jxgo", "the helm release name t ouse")
	command.Flags().StringVarP(&options.ChartName, "chart", "c", defaultChartName, "the chart name to use to install the git operator")
	command.Flags().StringVarP(&options.ChartVersion, "chart-version", "", "", "override the helm chart version used for the git operator")
	command.Flags().BoolVarP(&options.DryRun, "dry-run", "", false, "if enabled just display the helm command that will run but don't actually do anything")

	defaultBatchMode := false
	if os.Getenv("JX_BATCH_MODE") == "true" {
		defaultBatchMode = true
	}
	command.PersistentFlags().BoolVarP(&options.BatchMode, "batch-mode", "b", defaultBatchMode, "Runs in batch mode without prompting for user input")

	return command, options
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
		o.GitURL, err = findGitURLFromDir(o.Git(), o.Dir)
		if err != nil {
			return errors.Wrapf(err, "failed to detect the git URL from the directory %s", o.Dir)
		}
	}
	if o.GitURL != "" {
		o.GitURL, err = o.ensureHttpsURL(o.GitURL)
		if err != nil {
			return errors.Wrapf(err, "failed to ensure the git URL is a HTTPS UrL")
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
	args := []string{"install"}

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

func (o *Options) ensureHttpsURL(gitURL string) (string, error) {
	gitInfo, err := gits.ParseGitURL(gitURL)
	if err != nil {
		return gitURL, errors.Wrapf(err, "failed to parse git URL")
	}
	answer := gitInfo.HttpCloneURL()

	u, err := url.Parse(answer)
	if err != nil {
		return answer, errors.Wrapf(err, "failed to parse git URL %s", answer)
	}

	// lets check if we've already got a user and password
	if u.User != nil {
		user := u.User
		pwd, f := user.Password()
		if user.Username() != "" && pwd != "" && f {
			return answer, nil
		}
	}

	log.Logger().Infof("git clone URL is %s now adding the user/password so we can clone it", util.ColorInfo(answer))

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

func findGitURLFromDir(gitter gits.Gitter, dir string) (string, error) {
	_, gitConfDir, err := gitter.FindGitConfigDir(dir)
	if err != nil {
		return "", errors.Wrapf(err, "there was a problem obtaining the git config dir of directory %s", dir)
	}
	if gitConfDir == "" {
		return "", nil
	}
	return gitter.DiscoverUpstreamGitURL(gitConfDir)
}
