package preflight

import (
	. "github.com/onsi/gomega"
	"github.com/trilioData/tvk-plugins/internal"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func deleteSnapshotClass(name string) {
	volSnapVer, err := GetServerPreferredVersionForGroup(StorageSnapshotGroup, testClient.ClientSet)
	Expect(err).To(BeNil())
	vss := &unstructured.Unstructured{}
	vss.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   StorageSnapshotGroup,
		Version: volSnapVer,
		Kind:    internal.VolumeSnapshotClassKind,
	})
	err = testClient.RuntimeClient.Get(ctx, client.ObjectKey{
		Name: name,
	}, vss)
	Expect(err).To(BeNil())

	err = testClient.RuntimeClient.Delete(ctx, vss, client.GracePeriodSeconds(deletionGracePeriod))
	Expect(err).To(BeNil())
}

func createTestPod(name string) {
	pod := &unstructured.Unstructured{}
	pod.Object = map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": installNs,
		},
		"spec": map[string]interface{}{
			"containers": []map[string]interface{}{
				{
					"name":            BusyboxContainerName,
					"image":           BusyboxImageName,
					"command":         CommandSleep3600,
					"imagePullPolicy": corev1.PullIfNotPresent,
				},
			},
		},
	}
	pod.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind(internal.PodKind))
	err := testClient.RuntimeClient.Create(ctx, pod)
	Expect(err).To(BeNil())
}
