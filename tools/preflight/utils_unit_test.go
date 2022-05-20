package preflight

import (
	. "github.com/onsi/gomega"
	"github.com/trilioData/tvk-plugins/internal"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
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

func createTestPVC(pvcKey types.NamespacedName) error {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      pvcKey.Name,
			Namespace: pvcKey.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: &runOps.StorageClass,
			Resources: corev1.ResourceRequirements{
				Requests: map[corev1.ResourceName]resource.Quantity{
					corev1.ResourceStorage: runOps.PVCStorageRequest,
				},
			},
		},
	}

	return testClient.RuntimeClient.Create(ctx, pvc)
}

func createTestVolumeSnapsot(volSnapKey types.NamespacedName, pvcName, snapVer string) error {
	vs := &unstructured.Unstructured{}
	vs.Object = map[string]interface{}{
		"spec": map[string]interface{}{
			"volumeSnapshotClassName": testSnapshotClass,
			"source": map[string]string{
				"persistentVolumeClaimName": pvcName,
			},
		},
	}

	vs.SetName(volSnapKey.Name)
	vs.SetNamespace(volSnapKey.Namespace)
	vs.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   StorageSnapshotGroup,
		Version: snapVer,
		Kind:    internal.VolumeSnapshotKind,
	})

	return testClient.RuntimeClient.Create(ctx, vs)
}

func verifyTVKResourceLabels(obj *unstructured.Unstructured, nameSuffix string) {
	labelsMap, found, err := unstructured.NestedMap(obj.Object, "metadata", "labels")
	Expect(found).To(BeTrue())
	Expect(err).To(BeNil())

	val, ok := labelsMap[LabelPreflightRunKey]
	Expect(ok).To(BeTrue())
	Expect(val).To(Equal(nameSuffix))

	val, ok = labelsMap[LabelK8sName]
	Expect(ok).To(BeTrue())
	Expect(val).To(Equal(LabelK8sNameValue))

	val, ok = labelsMap[LabelTrilioKey]
	Expect(ok).To(BeTrue())
	Expect(val).To(Equal(LabelTvkPreflightValue))

	val, ok = labelsMap[LabelK8sPartOf]
	Expect(ok).To(BeTrue())
	Expect(val).To(Equal(LabelK8sPartOfValue))
}

func deletePod(podKey types.NamespacedName) {
	pod := &unstructured.Unstructured{}
	pod.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind(internal.PodKind))
	Expect(testClient.RuntimeClient.Get(ctx, podKey, pod)).To(BeNil())
	Expect(testClient.RuntimeClient.Delete(ctx, pod)).To(BeNil())
	Eventually(func() bool {
		err := testClient.RuntimeClient.Get(ctx, podKey, pod)
		return k8serrors.IsNotFound(err)
	}, timeout, interval).Should(BeTrue())
}

func deleteVolumeSnapshot(volSnapKey types.NamespacedName, volSnapGVK schema.GroupVersionKind) {
	volSnap := &unstructured.Unstructured{}
	volSnap.SetGroupVersionKind(volSnapGVK)
	Expect(testClient.RuntimeClient.Get(ctx, volSnapKey, volSnap)).To(BeNil())
	Expect(testClient.RuntimeClient.Delete(ctx, volSnap))
	Eventually(func() bool {
		err := testClient.RuntimeClient.Get(ctx, volSnapKey, volSnap)
		return k8serrors.IsNotFound(err)
	}, timeout, interval).Should(BeTrue())
}
