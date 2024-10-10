package reqhelpers

import (
	"slices"
	"strings"
)

const (
	GKE        = "gke"
	OKE        = "oke"
	EKS        = "eks"
	AKS        = "aks"
	AWS        = "aws"
	PKS        = "pks"
	IKS        = "iks"
	KUBERNETES = "kubernetes"
	OPENSHIFT  = "openshift"
	ICP        = "icp"
	JXINFRA    = "jx-infra"
	ALIBABA    = "alibaba"
)

// KubernetesProviders list of all available Kubernetes providers
var KubernetesProviders = []string{GKE, OKE, AKS, AWS, EKS, KUBERNETES, IKS, OPENSHIFT, JXINFRA, PKS, ICP, ALIBABA}

// KubernetesProviderOptions returns all the Kubernetes providers as a string
func KubernetesProviderOptions() string {
	values := append([]string{}, KubernetesProviders...)
	slices.Sort(values)
	return strings.Join(values, ", ")
}
