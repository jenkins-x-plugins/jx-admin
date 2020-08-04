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
	"github.com/jenkins-x/jx-helpers/pkg/kube/podlogs"
	"github.com/jenkins-x/jx-helpers/pkg/kube/pods"
	"github.com/jenkins-x/jx-helpers/pkg/stringhelpers"
	"github.com/jenkins-x/jx-helpers/pkg/termcolor"
	"github.com/jenkins-x/jx-kube-client/pkg/kubeclient"
	logger "github.com/jenkins-x/jx-logging/pkg/log"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Options contains the command line arguments for this command
type Options struct {
	Namespace     string
	Selector      string
	ContainerName string
	Duration      time.Duration
	MaxPods       int
	NoTail        bool
	BatchMode     bool
	ErrOut        io.Writer
	Out           io.Writer
	KubeClient    kubernetes.Interface
}

var (
	cmdLong = templates.LongDesc(`
		Views the boot Job logs in the cluster

`)

	cmdExample = templates.Examples(`
* views the current boot logs
` + bashExample("log") + `
`)
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
	command.Flags().StringVarP(&options.Selector, "selector", "s", "app=jx-boot", "the selector of the boot Job pods")
	command.Flags().StringVarP(&options.ContainerName, "container", "c", "job", "the name of the container in the boot Job to log")
	command.Flags().IntVarP(&options.MaxPods, "max-pods", "", 3, "the maximum number of pods to wait for")
	command.Flags().DurationVarP(&options.Duration, "duration", "d", time.Minute*30, "how long to wait for a pod to be ready")

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
	selector := o.Selector
	containerName := o.ContainerName

	info := termcolor.ColorInfo
	logger.Logger().Infof("waiting for boot Job pod with selector %s in namespace %s", info(selector), info(ns))

	end := time.Now().Add(o.Duration)
	var foundPods []string
	for {
		pod, err := pods.WaitForPodSelectorToBeReady(client, ns, selector, o.Duration)
		if err != nil {
			return err
		}
		if pod == nil {
			return errors.Errorf("No pod found for namespace %s with selector %v", ns, selector)
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
		err = podlogs.TailLogs(ns, podName, containerName, o.ErrOut, o.Out)
		if err != nil {
			logger.Logger().Warnf("failed to tail log: %s", err.Error())
		}
		pod, err = client.CoreV1().Pods(ns).Get(podName, metav1.GetOptions{})
		if err != nil {
			return errors.Wrapf(err, "failed to get pod %s in namespace %s", podName, ns)
		}
		if pods.IsPodCompleted(pod) {
			logger.Logger().Infof("the Job pod %s has completed successfully", info(podName))
			return nil
		}
		logger.Logger().Warnf("Job pod %s is not completed but has status: %s", podName, pods.PodStatus(pod))

		if o.MaxPods > 0 && len(foundPods) >= o.MaxPods {
			logger.Logger().Warnf("processed the maximum allowed %d pods so terminating", len(foundPods))
			return nil
		}

		if time.Now().After(end) {
			return errors.Errorf("timed out after waiting for duration %s", o.Duration.String())
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
