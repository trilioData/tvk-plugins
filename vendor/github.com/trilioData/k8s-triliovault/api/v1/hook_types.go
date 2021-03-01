package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// HookExecution specifies the Hook required to quiesce or unquiesce the application
type HookExecution struct {
	// ExecAction is a Command to be executed as a part of Hook. Specifies the action to take.
	// Commands should include what shell to use or the commands and its args which will be able to
	// run without the shell.
	// User can provide multiple commands merged as a part of a single command in the ExecAction.
	// Shell Script Ex. ["/bin/bash", "-c", "echo hello > hello.txt && echo goodbye > goodbye.txt"]
	ExecAction *corev1.ExecAction `json:"execAction"`

	// IgnoreFailure is a boolean, if set to true all the failures will be ignored
	// both in pre and post hooks
	// Default is false.
	// +kubebuilder:validation:Optional
	IgnoreFailure bool `json:"ignoreFailure,omitempty"`

	// MaxRetryCount count will be used to retry hook execution within the time range specified by Timeout in `TimeoutSeconds` field.
	// Hook execution will be considered in error if it fails to complete within `MaxRetryCount`.
	// Each retry count will be run with timeout of `TimeoutSeconds` field.
	// Default is 0
	// +kubebuilder:validation:Maximum=10
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Optional
	MaxRetryCount uint8 `json:"maxRetryCount,omitempty"`

	// TimeoutSeconds is A Maximum allowed time in seconds for each retry count according to value set in
	// `MaxRetryCount` field to execute Hook.
	// timeout here is a hard timeout.
	// MaxRetryCount field is related to TimeoutSeconds, Meaning each retry count will run with a timeout of `TimeoutSeconds`.
	// The command needs to exit in that time, either with exit code 0 or non 0.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Maximum=300
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default:30  // marker not supported yet
	TimeoutSeconds *uint16 `json:"timeoutSeconds,omitempty"`
}

// HookSpec defines the desired state of Hook.
type HookSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// PreHook is the Hook executed to quiesce the application before backup operation
	PreHook HookExecution `json:"pre"`

	// PostHook is the Hook executed to unquiesce the application after backup operation
	PostHook HookExecution `json:"post"`
}

// HookStatus defines the observed state of Hook.
type HookStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
}

// +k8s:openapi-gen=true
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:storageversion

// Hook is the Schema for the hooks API.
type Hook struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   HookSpec   `json:"spec,omitempty"`
	Status HookStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// HookList contains a list of Hook.
type HookList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Hook `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Hook{}, &HookList{})
}
