package preflight

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"
)

const (
	v1beta1K8sVersion = "v1.19.0"
	v1K8sVersion      = "v1.20.0"
	timeout           = time.Second * 60
	interval          = time.Second * 1
)

func preflightTestCases(serverVersion string) {

	Describe("Preflight unit test cases", func() {

		AfterEach(func() {
			deleteAllVolumeSnapshotCRD(serverVersion)
		})

		Context("Preflight run command volume snapshot CRD test cases", func() {

			It("Should skip installation if all volume snapshot CRDs are present", func() {
				installVolumeSnapshotCRD(serverVersion, [3]bool{true, true, true})
				Expect(run.checkAndCreateVolumeSnapshotCRDs(ctx, serverVersion)).To(BeNil())
				checkVolumeSnapshotCRDExists(serverVersion)
			})

			for i, crd := range VolumeSnapshotCRDs {
				It(fmt.Sprintf("Should install missing volume snapshot CRD %s when it is not present", crd), func() {
					var volumeSnapshotCRDsToInstall = [3]bool{true, true, true}
					volumeSnapshotCRDsToInstall[i] = false
					installVolumeSnapshotCRD(serverVersion, volumeSnapshotCRDsToInstall)
					Expect(run.checkAndCreateVolumeSnapshotCRDs(ctx, serverVersion)).To(BeNil())
					checkVolumeSnapshotCRDExists(serverVersion)
				})
			}

			It("Should install all volume snapshot CRDs when none of them are present", func() {
				deleteAllVolumeSnapshotCRD(serverVersion)
				Expect(run.checkAndCreateVolumeSnapshotCRDs(ctx, serverVersion)).To(BeNil())
				checkVolumeSnapshotCRDExists(serverVersion)
			})

		})

	})

}

var _ = Context("Preflight Unit Tests", func() {
	preflightTestCases(v1beta1K8sVersion)
	preflightTestCases(v1K8sVersion)
})

func installVolumeSnapshotCRD(version string, volumeSnapshotCRDToInstall [3]bool) {
	var crdObj client.Object

	for i, toBeInstalled := range volumeSnapshotCRDToInstall {
		crdObj = &apiextensionsv1.CustomResourceDefinition{}
		dirVersion := snapshotClassVersionV1
		if version == v1beta1K8sVersion {
			crdObj = &apiextensionsv1beta1.CustomResourceDefinition{}
			dirVersion = snapshotClassVersionV1Beta1
		}

		fileBytes, readErr := ioutil.ReadFile(filepath.Join(volumeSnapshotCRDYamlDir, dirVersion, VolumeSnapshotCRDs[i]+".yaml"))
		Expect(readErr).To(BeNil())

		Expect(yaml.Unmarshal(fileBytes, crdObj)).To(BeNil())

		if toBeInstalled {
			Expect(k8sClient.Create(ctx, crdObj)).To(BeNil())
			Eventually(func() error {
				var volSnapCRDObj client.Object
				volSnapCRDObj = &apiextensionsv1.CustomResourceDefinition{}
				if version == v1beta1K8sVersion {
					volSnapCRDObj = &apiextensionsv1beta1.CustomResourceDefinition{}
				}
				return k8sClient.Get(ctx, types.NamespacedName{Name: VolumeSnapshotCRDs[i]}, volSnapCRDObj)
			}, timeout, interval).ShouldNot(HaveOccurred())
		}
	}
}

func checkVolumeSnapshotCRDExists(version string) {
	var crdObj client.Object
	for _, crd := range VolumeSnapshotCRDs {
		crdObj = &apiextensionsv1.CustomResourceDefinition{}
		if version == v1beta1K8sVersion {
			crdObj = &apiextensionsv1beta1.CustomResourceDefinition{}
		}

		crdObj.SetName(crd)

		Eventually(func() error {
			return k8sClient.Get(ctx, types.NamespacedName{Name: crd}, crdObj)
		}, timeout, interval).ShouldNot(HaveOccurred())
	}
}

func deleteAllVolumeSnapshotCRD(version string) {
	var crdObj client.Object
	for _, crd := range VolumeSnapshotCRDs {
		crdObj = &apiextensionsv1.CustomResourceDefinition{}
		if version == v1beta1K8sVersion {
			crdObj = &apiextensionsv1beta1.CustomResourceDefinition{}
		}

		crdObj.SetName(crd)

		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: crd}, crdObj)
			if k8serrors.IsNotFound(err) {
				return true
			}
			Expect(k8sClient.Delete(ctx, crdObj)).To(BeNil())
			return false
		}, timeout, interval).Should(BeTrue())
	}
}
