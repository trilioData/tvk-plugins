package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:validation:Enum=Sequential;Parallel
// Mode is the enum for 2 modes of quiescing the application components i.e Sequential or Parallel
type Mode string

const (
	// Sequential defines the sequential quiescing mode and the quiescing sequence is required for this mode
	Sequential Mode = "Sequential"

	// Parallel defines the quiescing mode to be parallel which means
	// that the application components will be quiesced parallelly and hence the sequence will be ignored
	Parallel Mode = "Parallel"
)

// PrePostHookStatus defines Pre and Post hook execution status.
type PrePostHookStatus struct {

	// Status is the status for pre/post hook execution
	// +kubebuilder:validation:Enum=InProgress;Completed;Failed
	// +kubebuilder:validation:Optional
	Status Status `json:"status,omitempty"`

	// ExitStatus contains returned exit code and error trace after pre/post hook execution
	// +kubebuilder:validation:Optional
	ExitStatus string `json:"exitStatus,omitempty"`

	// RetryCount count used to retry hook execution within the time range specified by Timeout.
	// This is the actual number of times backup controller retried for pre/post hook execution if MaxRetryCount>0.
	// Default is 0
	// +kubebuilder:validation:Maximum=10
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Optional
	RetryCount uint8 `json:"retryCount,omitempty"`
}

// ContainerHookStatus defines hook execution status for a containers
type ContainerHookStatus struct {
	// ContainerName is container in which hooks are executed.
	// +kubebuilder:validation:Required
	ContainerName string `json:"containerName"`

	// PreHookStatus defines status for pre hooks
	// +kubebuilder:validation:Optional
	PreHookStatus PrePostHookStatus `json:"preHookStatus,omitempty"`

	// PostHookStatus defines status for post hooks
	// +kubebuilder:validation:Optional
	PostHookStatus PrePostHookStatus `json:"postHookStatus,omitempty"`
}

// PodHookStatus defines observed state for hooks
type PodHookStatus struct {
	// PodName is the single pod name from identified sets of pods filtered for hook config.
	// +kubebuilder:validation:Required
	PodName string `json:"podName"`

	// ContainerHookStatus defines status for filtered containers in a pod named 'PodName'
	// One Container can have multiple hook executions.
	// +kubebuilder:validation:MinItems=1
	ContainerHookStatus []ContainerHookStatus `json:"containerHookStatus"`
}

type Owner struct {
	// GroupVersionKind specifies GVK uniquely representing particular owner type.
	GroupVersionKind GroupVersionKind `json:"groupVersionKind"`

	// Name is name of owner
	Name string `json:"name"`
}

type HookTarget struct {
	// Owner specifies the parent for identified pods in PodHookStatus.
	// backup controller will fetch pods from Owner to execute the hooks.
	// Owner will be nil for pods with no owner.
	// +kubebuilder:validation:Optional
	Owner *Owner `json:"owner,omitempty"`

	// ContainerRegex identifies containers in identified pods to execute hooks.
	// +kubebuilder:validation:Optional
	ContainerRegex string `json:"containerRegex,omitempty"`

	// +kubebuilder:validation:Optional
	// +nullable:true
	// PodHookStatus specifies pre/post hook execution status for current backup.
	PodHookStatus []PodHookStatus `json:"podHookStatus,omitempty"`
}

// HookConfiguration contain's configuration for hook implementation.
type HookConfiguration struct {

	// MaxRetryCount is the maximum number of times pre/post hook execution can be retried.
	// MaxRetryCount will be equal to the RetryCount specified in Hook Spec.
	// +kubebuilder:validation:Maximum=10
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Optional
	MaxRetryCount uint8 `json:"maxRetryCount,omitempty"`

	// TimeoutSeconds is A Maximum allowed time in seconds to execute Hook.
	// timeout here is a hard timeout.
	// Meaning the command needs to exit in that time, either with exit code 0 or non 0.
	// hook execution will be considered in error if it fails to complete within timeout.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Maximum=300
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default:30 // marker not supported yet
	TimeoutSeconds *uint16 `json:"timeoutSeconds,omitempty"`

	// IgnoreFailure is a boolean, if set to true all the failures will be ignored for
	// both in pre and post hooks
	// Default is false.
	// +kubebuilder:validation:Optional
	IgnoreFailure bool `json:"ignoreFailure,omitempty"`
}

// HookPriority contain hook & their targeted resources
type HookPriority struct {
	// +kubebuilder:validation:Required
	// Hook is the object reference of the Hook resource which will be run while quiescing
	Hook *corev1.ObjectReference `json:"hook"`

	// PreHookConf defines how pre hook implementation will be handled
	PreHookConf *HookConfiguration `json:"preHookConf"`

	// PostHookConf defines how post hook implementation will be handled
	PostHookConf *HookConfiguration `json:"postHookConf"`

	// HookTarget defines targeting hook resources.
	HookTarget []HookTarget `json:"hookTarget"`
}

// HookComponentStatus indicates status of hook execution for backup/restore
type HookComponentStatus struct {
	// PodReadyWaitSeconds is the wait time for which hook execution waits before performing hook Quiescing/UnQuiescing
	// It is only applicable for  pods which are found in NotRunning state during hook execution
	// Default value is 120s, that will be set by webhook.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=600
	// +kubebuilder:default:120  // marker not supported yet
	PodReadyWaitSeconds *uint16 `json:"podReadyWaitSeconds,omitempty"`

	// +kubebuilder:validation:Optional
	// +nullable:true
	// HookPriorityStatuses specifies pre/post hook execution status for current backup.
	HookPriorityStatuses []HookPriorityStatus `json:"hookPriorityStatus,omitempty"`
}

// HookPriorityStatus defines observed state for hooks priority wise.
type HookPriorityStatus struct {
	// Priority defines priority for hooks.
	// backup controller will use `Priority` to determine sequence of hook execution.
	// In case of parallel Mode, priority will be same for all,
	// in case of sequential Mode, priority will be same for a group and not individual HookConfig Set.
	// Default Priority is 0.
	// +kubebuilder:validation:Minimum=0
	Priority uint8 `json:"priority"`

	// Hooks defines list of hooks with priority `Priority`.
	// +kubebuilder:validation:MinItems=1
	Hooks []HookPriority `json:"hooks"`
}

// HookInfo defines the config for hook action object reference to the matching regexes of pod and containers
type HookInfo struct {

	// +kubebuilder:validation:Required
	// Hook is the object reference of the Hook resource which will be run while quiescing
	Hook *corev1.ObjectReference `json:"hook"`

	// PodSelector will identify set of pods for hook config based on
	// either Labels or Regex pattern.
	PodSelector *PodSelector `json:"podSelector"`

	// ContainerRegex identifies containers for hook execution from pods which are filtered using PodSelector.
	// If not given then hooks will be executed in all the containers of the identified pods
	// +kubebuilder:validation:Optional
	ContainerRegex string `json:"containerRegex,omitempty"`
}

// HookConfig defines the sequence of hook actions and their associated pod-container regexes
type HookConfig struct {

	// +kubebuilder:validation:Optional
	// Mode can be sequential or parallel which defines the way hooks will be executed.
	// If mode is parallel, ignore the hook sequence.
	Mode `json:"mode,omitempty"`

	// PodReadyWaitSeconds is the wait time for which hook execution waits before performing hook Quiescing/UnQuiescing
	// It is only applicable for pods which are found in NotRunning state during hook execution
	// Default value is 120s, that will be set by webhook.
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=600
	// +kubebuilder:default:120  // marker not supported yet
	PodReadyWaitSeconds *uint16 `json:"podReadyWaitSeconds,omitempty"`

	// Hooks defines the config's for hook action object reference to the matching regexes of pod and containers
	// +kubebuilder:validation:MinItems=1
	Hooks []HookInfo `json:"hooks"`
}

// PodSelector selects pods for hook execution based on either Labels or Regex pattern.
// Both Labels & Regex can also specify
type PodSelector struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MinItems=1
	Labels []metav1.LabelSelector `json:"labels,omitempty"`

	// +kubebuilder:validation:Optional
	Regex string `json:"regex,omitempty"`
}
