package preflight

import (
	"github.com/trilioData/tvk-plugins/internal"

	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func deleteSnapshotClass(name string) {
	volSnapVer, err := GetServerPreferredVersionForGroup(StorageSnapshotGroup, k8sClient)
	Expect(err).To(BeNil())
	vss := &unstructured.Unstructured{}
	vss.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   StorageSnapshotGroup,
		Version: volSnapVer,
		Kind:    internal.VolumeSnapshotClassKind,
	})
	err = contClient.Get(ctx, client.ObjectKey{
		Name: name,
	}, vss)
	Expect(err).To(BeNil())

	err = contClient.Delete(ctx, vss, client.GracePeriodSeconds(deletionGracePeriod))
	Expect(err).To(BeNil())
}

func createTestPod(name, ns string, nodeSelOps map[string]string) *unstructured.Unstructured {
	pod := &unstructured.Unstructured{}
	pod.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": ns,
		},
		"spec": map[string]interface{}{
			"containers": []map[string]interface{}{
				{
					"name":            testContainerName,
					"image":           BusyboxImageName,
					"command":         CommandSleep3600,
					"imagePullPolicy": corev1.PullIfNotPresent,
				},
			},
			"nodeSelector": nodeSelOps,
		},
	}
	pod.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind(internal.PodKind))

	err := contClient.Create(ctx, pod)
	Expect(err).To(BeNil())

	Eventually(func() error {
		return contClient.Get(ctx, client.ObjectKey{
			Name:      name,
			Namespace: ns,
		}, pod)
	}, timeout, interval).Should(BeNil())

	return pod
}

func createTestPodStructured(name, ns string) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            BusyboxContainerName,
					Image:           BusyboxImageName,
					Command:         CommandSleep3600,
					ImagePullPolicy: corev1.PullIfNotPresent,
				},
			},
		},
	}
	pod, err := k8sClient.CoreV1().Pods(ns).Create(ctx, pod, metav1.CreateOptions{})
	Expect(err).To(BeNil())

	Eventually(func() error {
		_, err = k8sClient.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
		return err
	}, timeout, interval).Should(BeNil())

	return pod
}
