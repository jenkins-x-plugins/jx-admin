package clienthelpers

import "k8s.io/client-go/rest"

// IsInCluster tells if we are running incluster
func IsInCluster() bool {
	_, err := rest.InClusterConfig()
	return err == nil
}
