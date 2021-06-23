package trigger

import (
	"context"
	"fmt"

	"github.com/jenkins-x-plugins/jx-admin/pkg/bootjobs"
	"github.com/jenkins-x-plugins/jx-admin/pkg/rootcmd"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cobras/helper"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cobras/templates"
	"github.com/jenkins-x/jx-helpers/v3/pkg/kube"
	"github.com/jenkins-x/jx-helpers/v3/pkg/options"
	"github.com/jenkins-x/jx-helpers/v3/pkg/termcolor"
	"github.com/jenkins-x/jx-kube-client/v3/pkg/kubeclient"
	"github.com/jenkins-x/jx-logging/v3/pkg/log"
	logger "github.com/jenkins-x/jx-logging/v3/pkg/log"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Options contains the command line arguments for this command
type Options struct {
	options.BaseOptions

	Namespace   string
	JobSelector string
	CommitSHA   string
	KubeClient  kubernetes.Interface
}

const (
	// LabelCommitSHA the label added to git operator Jobs to indicate the commit sha
	LabelCommitSHA = "git-operator.jenkins.io/commit-sha"
)

var (
	info = termcolor.ColorInfo

	cmdLong = templates.LongDesc(`
		Triggers the latest boot Job to run again

`)

	cmdExample = templates.Examples(`
* trigger the boot job again
` + bashExample("trigger") + `
`)
)

// bashExample returns markdown for a bash script expression
func bashExample(cli string) string {
	return fmt.Sprintf("\n```bash \n%s %s\n```\n", rootcmd.BinaryName, cli)
}

// NewCmdJobTrigger creates the new command
func NewCmdJobTrigger() (*cobra.Command, *Options) {
	options := &Options{}
	command := &cobra.Command{
		Use:     "trigger",
		Short:   "triggers the latest boot Job to run again",
		Aliases: []string{"rerun"},
		Long:    cmdLong,
		Example: cmdExample,
		Run: func(command *cobra.Command, args []string) {
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	command.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "the namespace where the boot jobs run. If not specified it will look in: jx-git-operator and jx")
	command.Flags().StringVarP(&options.JobSelector, "selector", "s", "app=jx-boot", "the selector of the boot Job pods")
	command.Flags().StringVarP(&options.CommitSHA, "commit-sha", "", "", "the git commit SHA to filter jobs by")

	options.BaseOptions.AddBaseFlags(command)

	return command, options
}

func (o *Options) Run() error {
	err := o.Validate()
	if err != nil {
		return err
	}

	client := o.KubeClient
	selector := o.JobSelector

	ns, err := bootjobs.FindGitOperatorNamespace(client, o.Namespace)
	if err != nil {
		return errors.Wrapf(err, "failed to find the git operator namespace")
	}

	jobs, err := bootjobs.GetSortedJobs(client, ns, selector, o.CommitSHA)
	if err != nil {
		return errors.Wrapf(err, "failed to get jobs")
	}

	if len(jobs) == 0 {
		logger.Logger().Infof("there are no git operator jobs found in namespace %s", ns)
	}

	job := jobs[0]
	job.Labels["git-operator.jenkins.io/rerun"] = "true"
	ctx := context.Background()
	_, err = client.BatchV1().Jobs(ns).Update(ctx, &job, metav1.UpdateOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to update Job %s in namespace %s", job.Name, job.Namespace)
	}
	log.Logger().Infof("marked Job %s to be rerun. You can view the logs via: %s", info(job.Name), info("jx admin log"))
	return nil
}

// Validate verifies the settings are correct and we can lazy create any required resources
func (o *Options) Validate() error {
	var err error
	o.KubeClient, err = kube.LazyCreateKubeClientWithMandatory(o.KubeClient, true)
	if err != nil {
		return errors.Wrapf(err, "failed to create kubernetes client")
	}
	if o.Namespace == "" {
		o.Namespace, err = kubeclient.CurrentNamespace()
		if err != nil {
			return errors.Wrapf(err, "failed to detect current namespace. Try supply --namespace")
		}
	}
	return nil
}
