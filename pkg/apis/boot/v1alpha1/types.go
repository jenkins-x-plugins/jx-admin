package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Boot represents the boot configuration
//
// +k8s:openapi-gen=true
type Boot struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata"`

	// Spec holds the boot configuration
	// +optional
	Spec BootSpec `json:"spec"`
}

// BootSpec defines the desired state of Boot.
type BootSpec struct {
	// PipelineBotUser the pipeline bot user used to clone the boot git repository and run pipelines
	PipelineBotUser string `json:"pipelineBotUser,omitempty"`

	// SecretManager the kind of secrets manager
	SecretManager string `json:"secretManager,omitempty"`
}

// BootList contains a list of Boot
//
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type BootList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Boot `json:"items"`
}

// UsingExternalSecrets returns true if the boot config is using external secrets so that
// we don't need the $JX_SECRETS_YAML environment variable
func (b *Boot) UsingExternalSecrets() bool {
	return b != nil && b.Spec.SecretManager == "external-secrets"
}
