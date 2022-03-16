package preflight

import (
	. "github.com/onsi/gomega"
	"github.com/trilioData/tvk-plugins/internal"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func createStorageClass(name, provisioner string) {
	sc := &unstructured.Unstructured{}
	sc.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": name,
		},
		"provisioner":       provisioner,
		"reclaimPolicy":     reclaimDelete,
		"volumeBindingMode": bindingModeImmediate,
	}
	scVer, err := GetServerPreferredVersionForGroup(storageClassGroup, k8sClient)
	Expect(err).To(BeNil())
	sc.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   storageClassGroup,
		Version: scVer,
		Kind:    internal.StorageClassKind,
	})
	err = contClient.Create(ctx, sc)
	Expect(err).To(BeNil())
}

func deleteStorageClass(name string) {
	scVer, err := GetServerPreferredVersionForGroup(storageClassGroup, k8sClient)
	Expect(err).To(BeNil())
	sc := &unstructured.Unstructured{}
	sc.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   storageClassGroup,
		Version: scVer,
		Kind:    internal.StorageClassKind,
	})
	err = contClient.Get(ctx, client.ObjectKey{
		Name: name,
	}, sc)
	Expect(err).To(BeNil())

	err = contClient.Delete(ctx, sc, client.GracePeriodSeconds(deletionGracePeriod))
	Expect(err).To(BeNil())
}

func createSnapshotClass(driver string) {
	vss := &unstructured.Unstructured{}
	vss.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": testSnapshotClass,
		},
		"driver":         driver,
		"deletionPolicy": reclaimDelete,
	}
	volSnapVer, err := GetServerPreferredVersionForGroup(StorageSnapshotGroup, k8sClient)
	Expect(err).To(BeNil())
	vss.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   StorageSnapshotGroup,
		Version: volSnapVer,
		Kind:    internal.VolumeSnapshotClassKind,
	})

	err = contClient.Create(ctx, vss)
	Expect(err).To(BeNil())
}

func deleteSnapshotClass() {
	volSnapVer, err := GetServerPreferredVersionForGroup(StorageSnapshotGroup, k8sClient)
	Expect(err).To(BeNil())
	vss := &unstructured.Unstructured{}
	vss.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   StorageSnapshotGroup,
		Version: volSnapVer,
		Kind:    internal.VolumeSnapshotClassKind,
	})
	err = contClient.Get(ctx, client.ObjectKey{
		Name: testSnapshotClass,
	}, vss)
	Expect(err).To(BeNil())

	err = contClient.Delete(ctx, vss, client.GracePeriodSeconds(deletionGracePeriod))
	Expect(err).To(BeNil())
}

func createService(name, ns string) {
	svc := &unstructured.Unstructured{}
	svc.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": ns,
		},
		"spec": map[string]interface{}{
			"ports": []map[string]interface{}{
				{
					"name":       "https",
					"port":       443,
					"protocol":   "TCP",
					"targetPort": 443,
				},
			},
			"type": "ClusterIP",
		},
	}

	svc.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    internal.ServiceKind,
	})

	err := contClient.Create(ctx, svc)
	Expect(err).To(BeNil())
}

func deleteService(name, ns string) {
	svc := &unstructured.Unstructured{}
	svc.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    internal.ServiceKind,
	})
	err := contClient.Get(ctx, client.ObjectKey{
		Name:      name,
		Namespace: ns,
	}, svc)
	Expect(err).To(BeNil())

	err = contClient.Delete(ctx, svc, client.GracePeriodSeconds(deletionGracePeriod))
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

	return pod
}

func deleteTestPod(name, ns string) {
	pod := &unstructured.Unstructured{}
	pod.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    internal.PodKind,
	})
	err := contClient.Get(ctx, client.ObjectKey{
		Name:      name,
		Namespace: ns,
	}, pod)
	Expect(err).To(BeNil())

	err = contClient.Delete(ctx, pod, client.GracePeriodSeconds(deletionGracePeriod))
	Expect(err).To(BeNil())

	Eventually(func() int {
		err = contClient.Delete(ctx, pod, client.GracePeriodSeconds(deletionGracePeriod))
		if err == nil {
			return 1
		}
		return 0
	}, timeout, interval).Should(Equal(0))
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

	return pod
}
