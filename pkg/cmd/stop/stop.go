package stop

import (
	"context"

	"github.com/jenkins-x-plugins/jx-admin/pkg/bootjobs"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cobras/helper"
	"github.com/jenkins-x/jx-helpers/v3/pkg/cobras/templates"
	"github.com/jenkins-x/jx-helpers/v3/pkg/kube"
	"github.com/jenkins-x/jx-helpers/v3/pkg/options"
	"github.com/jenkins-x/jx-helpers/v3/pkg/termcolor"
	"github.com/jenkins-x/jx-kube-client/v3/pkg/kubeclient"
	"github.com/jenkins-x/jx-logging/v3/pkg/log"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Options contains the command line arguments for this command
type Options struct {
	options.BaseOptions

	Namespace   string
	JobSelector string
	KubeClient  kubernetes.Interface
}

var (
	info = termcolor.ColorInfo

	cmdLong = templates.LongDesc(`
		Stops the currently running boot Job.

		It works by setting spec.suspend=true in the job.
`)
)

// NewCmdJobStep creates the new command
func NewCmdJobStop() (*cobra.Command, *Options) {
	options := &Options{}
	command := &cobra.Command{
		Use:     "stop",
		Short:   "stops the currently running boot Job",
		Aliases: []string{"suspend"},
		Long:    cmdLong,
		Run: func(command *cobra.Command, args []string) {
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	command.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "the namespace where the boot jobs run. If not specified it will look in: jx-git-operator and jx")
	command.Flags().StringVarP(&options.JobSelector, "selector", "s", "app=jx-boot", "the selector of the boot Job pods")

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
		log.Logger().WithError(err).Errorf("failed to find the git operator namespace")
		return nil
	}

	jobs, err := bootjobs.GetSortedJobs(client, ns, selector, "")
	if err != nil {
		log.Logger().WithError(err).Errorf("failed to get jobs")
		return nil
	}

	if len(jobs) == 0 {
		log.Logger().Warnf("there are no boot jobs found in namespace %s", ns)
		return nil
	}

	job := jobs[0]
	if IsJobDone(&job) {
		log.Logger().Warnf("there is no running boot job in namespace %s", ns)
		return nil
	}
	ctx := context.Background()
	suspend := true
	job.Spec.Suspend = &suspend
	_, err = client.BatchV1().Jobs(ns).Update(ctx, &job, metav1.UpdateOptions{})
	if err != nil {
		return errors.Wrapf(err, "failed to update Job %s in namespace %s", job.Name, job.Namespace)
	}
	log.Logger().Infof("marked Job %s to be stopped.", info(job.Name))
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

// IsJobDone returns true if the job has completed
func IsJobDone(r *batchv1.Job) bool {
	for _, con := range r.Status.Conditions {
		if con.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
