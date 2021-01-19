package bootjobs

import (
	"context"
	"github.com/jenkins-x/jx-logging/v3/pkg/log"
	"sort"
	"strings"
	"time"

	"github.com/jenkins-x/jx-helpers/v3/pkg/stringhelpers"
	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// GetSortedJobs gets the boot jobs with an optional commit sha filter
func GetSortedJobs(client kubernetes.Interface, ns string, selector string, commitSHA string) ([]batchv1.Job, error) {
	jobList, err := client.BatchV1().Jobs(ns).List(context.TODO(), metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil && !apierrors.IsNotFound(err) {
		return nil, errors.Wrapf(err, "failed to list jobList in namespace %s selector %s", ns, selector)
	}

	answer := jobList.Items
	if commitSHA != "" {
		var filtered []batchv1.Job
		for _, job := range answer {
			labels := job.Labels
			if labels != nil {
				sha := labels[LabelCommitSHA]
				if strings.Contains(sha, commitSHA) {
					filtered = append(filtered, job)
				}
			}
		}
		answer = filtered
	}

	sort.Slice(answer, func(i, j int) bool {
		j1 := answer[i]
		j2 := answer[j]
		return j2.CreationTimestamp.Before(&j1.CreationTimestamp)
	})
	return answer, nil
}

func WaitForGitOperatorNamespace(client kubernetes.Interface, ns string, timeout time.Duration) (string, error) {
	timeEnd := time.Now().Add(timeout)
	lastError := ""
	for {
		ns, err := FindGitOperatorNamespace(client, ns)
		if err == nil && ns != "" {
			return ns, nil
		}

		if err != nil {
			errText := err.Error()
			if errText != lastError {
				log.Logger().Warnf(errText)
				lastError = errText
			}
		}

		if time.Now().After(timeEnd) {
			return "", errors.Errorf("timed out after waiting for duration %s", timeout.String())
		}
		time.Sleep(time.Second)
	}
}

// FindGitOperatorNamespace finds the git operator namespace
func FindGitOperatorNamespace(client kubernetes.Interface, namespace string) (string, error) {
	namespaces := []string{"jx", "jx-git-operator"}
	if stringhelpers.StringArrayIndex(namespaces, namespace) < 0 {
		namespaces = append(namespaces, namespace)
	}
	name := "jx-git-operator"
	for _, ns := range namespaces {
		_, err := client.AppsV1().Deployments(ns).Get(context.TODO(), name, metav1.GetOptions{})
		if err == nil {
			return ns, nil
		}
		if !apierrors.IsNotFound(err) {
			return ns, errors.Wrapf(err, "failed to find Deployment %s in namespace %s", name, ns)
		}
	}
	return namespace, errors.Errorf("failed to find Deployment %s in namespaces %s", name, strings.Join(namespaces, ", "))
}
