package wait

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	client "k8s.io/client-go/kubernetes"
)

type Response struct {
	//  boolean flag.
	// True signifies that the pod has reached the desired `PodCondition` within timeout value.
	// False signifies that pod could not achieve the desired `PodCondition` within timeout value.
	ReachedCondn bool

	Err error
}

// PodWaitOptions  waits until the pod reaches the desired condition or times-out.
type PodWaitOptions struct {
	//  Name - pod name
	Name      string
	Namespace string
	//  timeout - time to wait till pod reaches the desired condition
	Timeout      time.Duration
	PodCondition corev1.PodConditionType
	ClientSet    *client.Clientset
}

func (o *PodWaitOptions) WaitOnPod(ctx context.Context, retryBackoff wait.Backoff) *Response {
	retErr := wait.ExponentialBackoff(retryBackoff, func() (done bool, err error) {
		pod, err := o.ClientSet.CoreV1().Pods(o.Namespace).Get(ctx, o.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		podConds := pod.Status.Conditions
		for i := range podConds {
			if podConds[i].Status == corev1.ConditionTrue && podConds[i].Type == o.PodCondition {
				return true, nil
			}
		}
		return false, nil
	})

	if retErr != nil {
		return &Response{
			ReachedCondn: false,
			Err:          retErr,
		}
	}

	return &Response{
		ReachedCondn: true,
		Err:          nil,
	}
}
