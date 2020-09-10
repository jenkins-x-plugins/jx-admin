package joblog

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jenkins-x/jx-admin/pkg/common"
	"github.com/jenkins-x/jx-admin/pkg/rootcmd"
	"github.com/jenkins-x/jx-helpers/pkg/cobras/helper"
	"github.com/jenkins-x/jx-helpers/pkg/cobras/templates"
	"github.com/jenkins-x/jx-helpers/pkg/kube"
	"github.com/jenkins-x/jx-helpers/pkg/kube/jobs"
	"github.com/jenkins-x/jx-helpers/pkg/kube/podlogs"
	"github.com/jenkins-x/jx-helpers/pkg/kube/pods"
	"github.com/jenkins-x/jx-helpers/pkg/stringhelpers"
	"github.com/jenkins-x/jx-helpers/pkg/termcolor"
	"github.com/jenkins-x/jx-kube-client/pkg/kubeclient"
	logger "github.com/jenkins-x/jx-logging/pkg/log"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Options contains the command line arguments for this command
type Options struct {
	Namespace           string
	JobSelector         string
	GitOperatorSelector string
	ContainerName       string
	CommitSHA           string
	Duration            time.Duration
	PollPeriod          time.Duration
	NoTail              bool
	BatchMode           bool
	ShaMode             bool
	ErrOut              io.Writer
	Out                 io.Writer
	KubeClient          kubernetes.Interface
	timeEnd             time.Time
	podStatusMap        map[string]string
}

const (
	// LabelCommitSHA the label added to git operator Jobs to indicate the commit sha
	LabelCommitSHA = "git-operator.jenkins.io/commit-sha"
)

var (
	cmdLong = templates.LongDesc(`
		Views the boot Job logs in the cluster

`)

	cmdExample = templates.Examples(`
* views the current boot logs
` + bashExample("log") + `
`)

	info = termcolor.ColorInfo
)

// bashExample returns markdown for a bash script expression
func bashExample(cli string) string {
	return fmt.Sprintf("\n```bash \n%s %s\n```\n", rootcmd.BinaryName, cli)
}

// NewCmdJobLog creates the new command
func NewCmdJobLog() (*cobra.Command, *Options) {
	options := &Options{}
	command := &cobra.Command{
		Use:     "log",
		Short:   "views the boot Job logs in the cluster",
		Aliases: []string{"logs"},
		Long:    cmdLong,
		Example: cmdExample,
		Run: func(command *cobra.Command, args []string) {
			err := options.Run()
			helper.CheckErr(err)
		},
	}
	command.Flags().StringVarP(&options.Namespace, "namespace", "n", common.DefaultOperatorNamespace, "the namespace where the boot jobs run")
	command.Flags().StringVarP(&options.JobSelector, "selector", "s", "app=jx-boot", "the selector of the boot Job pods")
	command.Flags().StringVarP(&options.GitOperatorSelector, "git-operator-selector", "g", "app=jx-git-operator", "the selector of the git operator pod")
	command.Flags().StringVarP(&options.ContainerName, "container", "c", "job", "the name of the container in the boot Job to log")
	command.Flags().StringVarP(&options.CommitSHA, "commit-sha", "", "", "the git commit SHA of the git repository to query the boot Job for")
	command.Flags().BoolVarP(&options.ShaMode, "sha-mode", "", false, "if --commit-sha is not specified then default the git commit SHA from $ and fail if it could not be found")
	command.Flags().DurationVarP(&options.Duration, "duration", "d", time.Minute*30, "how long to wait for a Job to be active and a Pod to be ready")
	command.Flags().DurationVarP(&options.PollPeriod, "poll", "", time.Second*1, "duration between polls for an active Job or Pod")

	defaultBatchMode := false
	if os.Getenv("JX_BATCH_MODE") == "true" {
		defaultBatchMode = true
	}
	command.PersistentFlags().BoolVarP(&options.BatchMode, "batch-mode", "b", defaultBatchMode, "Runs in batch mode without prompting for user input")

	return command, options
}

func (o *Options) Run() error {
	err := o.Validate()
	if err != nil {
		return err
	}
	ns := o.Namespace
	client := o.KubeClient
	selector := o.JobSelector
	containerName := o.ContainerName

	o.timeEnd = time.Now().Add(o.Duration)

	logger.Logger().Infof("waiting for the Git Operator to be ready in namespace %s...", info(ns))

	goPod, err := pods.WaitForPodSelectorToBeReady(client, ns, o.GitOperatorSelector, o.Duration)
	if err != nil {
		return errors.Wrapf(err, "failed waiting for the git operator pod to be ready in namespace %s with selector %s", ns, o.GitOperatorSelector)
	}
	if goPod == nil {
		logger.Logger().Infof(`Could not find the git operator. 

Are you sure you have installed the git operator?

See: https://jenkins-x.io/docs/v3/guides/operator/

`)
		return errors.Wrapf(err, "no git operator pod to be ready in namespace %s with selector %s", ns, o.GitOperatorSelector)
	}
	logger.Logger().Infof("the Git Operator is running in pod %s\n\n", info(goPod.Name))

	info := termcolor.ColorInfo
	if o.CommitSHA != "" {
		logger.Logger().Infof("waiting for boot Job pod with selector %s in namespace %s for commit SHA %s...", info(selector), info(ns), info(o.CommitSHA))

	} else {
		logger.Logger().Infof("waiting for boot Job pod with selector %s in namespace %s...", info(selector), info(ns))
	}

	job, err := o.waitForLatestJob(client, ns, selector)
	if err != nil {
		return errors.Wrapf(err, "failed to wait for active Job in namespace %s with selector %v", ns, selector)
	}

	logger.Logger().Infof("waiting for Job %s to complete...", info(job.Name))

	var foundPods []string
	for {
		complete, pod, err := o.waitForJobCompleteOrPodReady(client, ns, selector, job.Name)
		if err != nil {
			return err
		}
		if complete {
			return nil
		}
		if pod == nil {
			return errors.Errorf("No pod found for namespace %s with selector %v", ns, selector)
		}

		if time.Now().After(o.timeEnd) {
			return errors.Errorf("timed out after waiting for duration %s", o.Duration.String())
		}

		// lets verify the container name
		err = verifyContainerName(pod, containerName)
		if err != nil {
			return err
		}
		podName := pod.Name
		if stringhelpers.StringArrayIndex(foundPods, podName) < 0 {
			foundPods = append(foundPods, podName)
		}
		logger.Logger().Infof("\ntailing boot Job pod %s\n\n", info(podName))

		err = podlogs.TailLogs(ns, podName, containerName, o.ErrOut, o.Out)
		if err != nil {
			logger.Logger().Warnf("failed to tail log: %s", err.Error())
		}
		pod, err = client.CoreV1().Pods(ns).Get(podName, metav1.GetOptions{})
		if err != nil {
			return errors.Wrapf(err, "failed to get pod %s in namespace %s", podName, ns)
		}
		if pods.IsPodCompleted(pod) {
			if pods.IsPodSucceeded(pod) {
				logger.Logger().Infof("boot Job pod %s has %s", info(podName), info("Succeeded"))
			} else {
				logger.Logger().Infof("boot Job pod %s has %s", info(podName), termcolor.ColorError(string(pod.Status.Phase)))
			}
		} else if pod.DeletionTimestamp != nil {
			logger.Logger().Infof("boot Job pod %s is %s", info(podName), termcolor.ColorWarning("Terminating"))
		}
	}
}

// Validate verifies the settings are correct and we can lazy create any required resources
func (o *Options) Validate() error {
	if o.NoTail {
		return nil
	}
	if o.ErrOut == nil {
		o.ErrOut = os.Stderr
	}
	if o.Out == nil {
		o.Out = os.Stdout
	}
	if o.ShaMode && o.CommitSHA == "" {
		o.CommitSHA = os.Getenv("PULL_BASE_SHA")
		if o.CommitSHA == "" {
			return errors.Errorf("you have specified --sha-mode but no $PULL_BASE_SHA is defined or --commit-sha option supplied")
		}
	}

	var err error
	o.KubeClient, err = kube.LazyCreateKubeClient(o.KubeClient)
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

func (o *Options) waitForLatestJob(client kubernetes.Interface, ns, selector string) (*batchv1.Job, error) {
	for {
		job, err := o.getLatestJob(client, ns, selector)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to ")
		}

		if job != nil {
			if o.CommitSHA != "" || !jobs.IsJobFinished(job) {
				return job, nil
			}
		}

		if time.Now().After(o.timeEnd) {
			return nil, errors.Errorf("timed out after waiting for duration %s", o.Duration.String())
		}
		time.Sleep(o.PollPeriod)
	}
}

func (o *Options) waitForJobCompleteOrPodReady(client kubernetes.Interface, ns, selector, jobName string) (bool, *corev1.Pod, error) {
	if o.podStatusMap == nil {
		o.podStatusMap = map[string]string{}
	}

	for {
		complete, _, err := o.checkIfJobComplete(client, ns, jobName)
		if err != nil {
			return false, nil, errors.Wrapf(err, "failed to check for Job %s complete", jobName)
		}
		if complete {
			return true, nil, nil
		}

		pod, err := pods.GetReadyPodForSelector(client, ns, selector)
		if err != nil {
			return false, pod, errors.Wrapf(err, "failed to query ready pod in namespace %s with selector %s", ns, selector)
		}
		if pod != nil {
			status := pods.PodStatus(pod)
			if o.podStatusMap[pod.Name] != status && !pods.IsPodCompleted(pod) && pod.DeletionTimestamp == nil {
				logger.Logger().Infof("pod %s has status %s", termcolor.ColorInfo(pod.Name), termcolor.ColorInfo(status))
				o.podStatusMap[pod.Name] = status
			}
			if pods.IsPodReady(pod) {
				return false, pod, nil
			}
		}

		if time.Now().After(o.timeEnd) {
			return false, nil, errors.Errorf("timed out after waiting for duration %s", o.Duration.String())
		}
		time.Sleep(o.PollPeriod)
	}
}

func (o *Options) getLatestJob(client kubernetes.Interface, ns, selector string) (*batchv1.Job, error) {
	jobList, err := client.BatchV1().Jobs(ns).List(metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, errors.Wrapf(err, "failed to list jobList in namespace %s selector %s", ns, selector)
	}
	if len(jobList.Items) == 0 {
		return nil, nil
	}

	if o.CommitSHA != "" {
		for i := 0; i < len(jobList.Items); i++ {
			job := &jobList.Items[i]
			labels := job.Labels
			if labels != nil {
				if o.CommitSHA == labels[LabelCommitSHA] {
					return job, nil
				}
			}
		}
		return nil, nil
	}

	// lets find the newest job...
	latest := jobList.Items[0]
	for i := 1; i < len(jobList.Items); i++ {
		job := jobList.Items[i]
		if job.CreationTimestamp.After(latest.CreationTimestamp.Time) {
			latest = job
		}
	}
	return &latest, nil
}

func (o *Options) checkIfJobComplete(client kubernetes.Interface, ns, name string) (bool, *batchv1.Job, error) {
	job, err := client.BatchV1().Jobs(ns).Get(name, metav1.GetOptions{})
	if job == nil || err != nil {
		return false, nil, errors.Wrapf(err, "failed to list jobList in namespace %s name %s", ns, name)
	}
	if jobs.IsJobFinished(job) {
		if jobs.IsJobSucceeded(job) {
			logger.Logger().Infof("boot Job %s has %s", info(job.Name), info("Succeeded"))
			return true, job, nil
		}
		logger.Logger().Infof("boot Job %s has %s", info(job.Name), termcolor.ColorError("Failed"))
		return true, job, nil
	}
	logger.Logger().Debugf("boot Job %s is not completed yet", info(job.Name))
	return false, job, nil
}

func verifyContainerName(pod *corev1.Pod, name string) error {
	var names []string
	for i := range pod.Spec.Containers {
		if pod.Spec.Containers[i].Name == name {
			return nil
		}
		names = append(names, pod.Spec.Containers[i].Name)
	}
	sort.Strings(names)
	return errors.Errorf("invalid container name %s for pod %s. Available names: %s", name, pod.Name, strings.Join(names, ", "))
}
