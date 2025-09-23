package cmd

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	goclient "k8s.io/client-go/kubernetes"
	restclient "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/yaml"

	"github.com/trilioData/tvk-plugins/internal"
	logcollector "github.com/trilioData/tvk-plugins/tools/log-collector"
)

var execCallCount int

type fakeExec struct{}

type fakeExecEmpty struct{}

func (f *fakeExec) Stream(opts remotecommand.StreamOptions) error {
	execCallCount++

	// First call is for ls -la (directory check), second call is for tar
	switch execCallCount {
	case 1:
		// Simulate ls -la output showing directory has files
		lsOutput := `total 8
drwxr-xr-x 2 root root 4096 Jan 1 00:00 .
drwxr-xr-x 3 root root 4096 Jan 1 00:00 ..
-rw-r--r-- 1 root root   10 Jan 1 00:00 sample.log
-rw-r--r-- 1 root root   15 Jan 1 00:00 another.log.gz
`
		if opts.Stdout != nil {
			_, _ = opts.Stdout.Write([]byte(lsOutput))
		}
		return nil
	case 2:
		// Write a small tar archive to opts.Stdout
		if opts.Stdout == nil {
			return nil
		}
		tw := tar.NewWriter(opts.Stdout)
		defer func() { _ = tw.Close() }()

		// Directory and file: logs/a.txt
		_ = tw.WriteHeader(&tar.Header{Name: "logs/", Mode: 0755, Typeflag: tar.TypeDir})
		dataA := []byte("alpha")
		_ = tw.WriteHeader(&tar.Header{Name: "logs/a.txt", Mode: 0644, Size: int64(len(dataA))})
		_, _ = tw.Write(dataA)

		// Directory and file: nested/b.log
		_ = tw.WriteHeader(&tar.Header{Name: "nested/", Mode: 0755, Typeflag: tar.TypeDir})
		dataB := []byte("bravo")
		_ = tw.WriteHeader(&tar.Header{Name: "nested/b.log", Mode: 0644, Size: int64(len(dataB))})
		_, _ = tw.Write(dataB)
		return nil
	}
	return nil
}

func (f *fakeExecEmpty) Stream(opts remotecommand.StreamOptions) error {
	// Simulate ls command failing (directory not found)
	if opts.Stderr != nil {
		_, _ = opts.Stderr.Write([]byte("ls: /var/log/triliovault: No such file or directory"))
	}
	return fmt.Errorf("command terminated with exit code 2")
}

var _ = Describe("log collector cmd helper unit tests", func() {

	Context("Initialization of log collector", func() {

		BeforeEach(func() {
			internal.KubeConfigDefault = os.Getenv(internal.KubeconfigEnv)
		})

		It("Should initialize log collector objects when valid kubeconfig file path is provided", func() {

			command := logCollectorCommand()
			err := command.PersistentPreRunE(command, []string{})
			Expect(err).Should(BeNil())
			Expect(logCollector.KubeConfig).ShouldNot(BeEmpty())
			Expect(logCollector.Clustered).Should(BeTrue())
			Expect(logCollector.Namespaces).Should(BeEmpty())
			Expect(inputFileName).Should(BeEmpty())
			Expect(logCollector.DisClient).ShouldNot(BeNil())
			Expect(logCollector.K8sClient).ShouldNot(BeNil())
			Expect(logCollector.K8sClientSet).ShouldNot(BeNil())
			Expect(logCollector.GroupVersionKinds).Should(BeEmpty())
			Expect(logCollector.LabelSelectors).Should(BeEmpty())
			Expect(logCollector.OutputDir).ShouldNot(BeEmpty())
			Expect(logCollector.CleanOutput).Should(BeFalse())
		})

		It("Should read values.yaml file path if provided with correct format", func() {

			var newLogCollector logcollector.LogCollector
			command := logCollectorCommand()
			inputFileName = filepath.Join(testDataDir, testInputFile)
			err := command.PersistentPreRunE(command, []string{})
			Expect(err).Should(BeNil())
			log.Debug(logCollector)

			data, err := os.ReadFile(inputFileName)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(yaml.UnmarshalStrict(data, &newLogCollector)).ShouldNot(HaveOccurred())

			Expect(len(newLogCollector.GroupVersionKinds)).Should(Equal(len(logCollector.GroupVersionKinds)))
			Expect(len(newLogCollector.LabelSelectors)).Should(Equal(len(logCollector.LabelSelectors)))

			Expect(newLogCollector.CleanOutput).Should(Equal(logCollector.CleanOutput))
			Expect(newLogCollector.Clustered).Should(Equal(logCollector.Clustered))
			Expect(newLogCollector.Loglevel).Should(Equal(logCollector.Loglevel))

		})

		It("Should throw error for values.yaml file path if provided with incorrect format", func() {

			var newLogCollector logcollector.LogCollector
			inputFileName = filepath.Join(testDataDir, invalidTestInputFile)
			pErr := manageFileInputs()

			data, err := os.ReadFile(inputFileName)
			Expect(err).ShouldNot(HaveOccurred())
			newLCerr := yaml.UnmarshalStrict(data, &newLogCollector)
			Expect(pErr).Should(Equal(newLCerr))
		})

		It("Should filter duplicate values from gvk and label selector", func() {

			var newLogCollector logcollector.LogCollector
			inputFileName = filepath.Join(testDataDir, testInputFileDuplicate)
			err := manageFileInputs()
			Expect(err).Should(BeNil())
			log.Debug(logCollector)

			data, err := os.ReadFile(inputFileName)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(yaml.UnmarshalStrict(data, &newLogCollector)).ShouldNot(HaveOccurred())

			Expect(len(newLogCollector.GroupVersionKinds)).ShouldNot(Equal(len(logCollector.GroupVersionKinds)))
			Expect(len(newLogCollector.LabelSelectors)).ShouldNot(Equal(len(logCollector.LabelSelectors)))

			for idx := range logCollector.LabelSelectors {
				Expect(logCollector.LabelSelectors[idx].MatchLabels).Should(HaveKeyWithValue("app", "frontend"))
				Expect(logCollector.LabelSelectors[idx].MatchLabels).Should(HaveKeyWithValue("custom", "label"))
			}

			for idx := range logCollector.GroupVersionKinds {
				Expect(logCollector.GroupVersionKinds[idx].Version).Should(Equal("v1"))
				Expect(logCollector.GroupVersionKinds[idx].Kind).Should(Equal("pod"))
			}

			// check if version populates in pre run if not given
			Expect(newLogCollector.GroupVersionKinds[0].Version).Should(BeEmpty())
			Expect(logCollector.GroupVersionKinds[0].Version).ShouldNot(BeEmpty())
			Expect(logCollector.GroupVersionKinds[0].Version).Should(Equal("v1"))

			Expect(newLogCollector.CleanOutput).Should(Equal(logCollector.CleanOutput))
			Expect(newLogCollector.Clustered).Should(Equal(logCollector.Clustered))
			Expect(newLogCollector.Loglevel).Should(Equal(logCollector.Loglevel))
		})

		It("Should return error when kubeconfig file contains invalid data", func() {

			logCollector.KubeConfig = "invalid/path/to/kubeconfig"
			err := logCollector.InitializeKubeClients()
			Expect(err).ShouldNot(BeNil())
		})

		It("Should run using kubeconfig file mentioned in KUBECONFIG env when no "+
			"explicit kubeconfig file is provided", func() {
			testConfigPath := filepath.Join(projectRoot, "manual", "path", "to", "kubeconfig")
			Expect(internal.CopyFile(internal.KubeConfigDefault, testConfigPath)).Should(Succeed())
			Expect(os.Setenv(internal.KubeconfigEnv, testConfigPath)).Should(Succeed())
			logCollector.KubeConfig = ""
			command := logCollectorCommand()
			Expect(command.PersistentPreRunE(command, []string{})).Should(Succeed())
			Expect(logCollector.KubeConfig).Should(Equal(testConfigPath))
		})

		Context("Helper functions for reading log from pod volumePath", func() {
			It("Should decompress .gz files and skip invalid ones", func() {
				root := GinkgoT().TempDir()
				nested := filepath.Join(root, "ns", "pod")
				Expect(os.MkdirAll(nested, 0755)).To(Succeed())

				validGzPath := filepath.Join(nested, "sample.log.gz")
				f, err := os.Create(validGzPath)
				Expect(err).To(BeNil())
				gw := gzip.NewWriter(f)
				original := []byte("hello world\nthis is a test")
				_, err = gw.Write(original)
				Expect(err).To(BeNil())
				Expect(gw.Close()).To(Succeed())
				Expect(f.Close()).To(Succeed())

				invalidGzPath := filepath.Join(nested, "corrupt.log.gz")
				Expect(os.WriteFile(invalidGzPath, []byte("not a gzip stream"), 0644)).To(Succeed())

				nonGzPath := filepath.Join(nested, "plain.txt")
				Expect(os.WriteFile(nonGzPath, []byte("keep me"), 0644)).To(Succeed())

				Expect(logcollector.DecompressGzInDir(root)).To(Succeed())

				_, err = os.Stat(validGzPath)
				Expect(os.IsNotExist(err)).To(BeTrue())
				decompressedPath := strings.TrimSuffix(validGzPath, ".gz")
				data, err := os.ReadFile(decompressedPath)
				Expect(err).To(BeNil())
				Expect(string(data)).To(Equal(string(original)))

				_, err = os.Stat(invalidGzPath)
				Expect(err).To(BeNil())

				plain, err := os.ReadFile(nonGzPath)
				Expect(err).To(BeNil())
				Expect(string(plain)).To(Equal("keep me"))
			})

			It("Should copy directory from pod via exec tar stream and extract", func() {
				// Start envtest to get a real API server and config
				testEnv := &envtest.Environment{}
				cfg, err := testEnv.Start()
				Expect(err).To(BeNil())
				Expect(cfg).NotTo(BeNil())
				defer func() { _ = testEnv.Stop() }()

				cs, err := goclient.NewForConfig(cfg)
				Expect(err).To(BeNil())
				Expect(cs).NotTo(BeNil())

				// Create a namespace and a simple pod
				ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "lc-test-ns"}}
				_, err = cs.CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{})
				Expect(err).To(BeNil())

				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "lc-test-pod", Namespace: ns.Name},
					Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "c0", Image: "busybox"}}},
				}
				_, err = cs.CoreV1().Pods(ns.Name).Create(context.Background(), pod, metav1.CreateOptions{})
				Expect(err).To(BeNil())

				// Destination for extraction
				dest := GinkgoT().TempDir()

				// Reset exec call counter for this test
				execCallCount = 0

				// Stub the SPDY executor factory
				origFactory := logcollector.SpdyExecutorFactory
				logcollector.SpdyExecutorFactory = func(_ *restclient.Config, _ string, _ *url.URL) (logcollector.Executor, error) {
					return &fakeExec{}, nil
				}
				defer func() { logcollector.SpdyExecutorFactory = origFactory }()

				lc := &logcollector.LogCollector{K8sClientSet: cs, RestConfig: cfg}
				err = lc.CopyDirFromPod(ns.Name, pod.Name, "c0", internal.TriliovaultLogDir, dest)
				Expect(err).To(BeNil())

				b, err := os.ReadFile(filepath.Join(dest, "logs", "a.txt"))
				Expect(err).To(BeNil())
				Expect(string(b)).To(Equal("alpha"))
				b, err = os.ReadFile(filepath.Join(dest, "nested", "b.log"))
				Expect(err).To(BeNil())
				Expect(string(b)).To(Equal("bravo"))
			})

			It("Should skip copy when directory is empty or doesn't exist", func() {
				// Start envtest to get a real API server and config
				testEnv := &envtest.Environment{}
				cfg, err := testEnv.Start()
				Expect(err).To(BeNil())
				Expect(cfg).NotTo(BeNil())
				defer func() { _ = testEnv.Stop() }()

				cs, err := goclient.NewForConfig(cfg)
				Expect(err).To(BeNil())
				Expect(cs).NotTo(BeNil())

				// Create a namespace and a simple pod
				ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "lc-test-ns-empty"}}
				_, err = cs.CoreV1().Namespaces().Create(context.Background(), ns, metav1.CreateOptions{})
				Expect(err).To(BeNil())

				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{Name: "lc-test-pod-empty", Namespace: ns.Name},
					Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "c0", Image: "busybox"}}},
				}
				_, err = cs.CoreV1().Pods(ns.Name).Create(context.Background(), pod, metav1.CreateOptions{})
				Expect(err).To(BeNil())

				// Destination for extraction
				dest := GinkgoT().TempDir()

				// Reset exec call counter for this test
				execCallCount = 0

				// Stub the SPDY executor factory to simulate directory not found
				origFactory := logcollector.SpdyExecutorFactory
				logcollector.SpdyExecutorFactory = func(_ *restclient.Config, _ string, _ *url.URL) (logcollector.Executor, error) {
					return &fakeExecEmpty{}, nil
				}
				defer func() { logcollector.SpdyExecutorFactory = origFactory }()

				lc := &logcollector.LogCollector{K8sClientSet: cs, RestConfig: cfg}
				err = lc.CopyDirFromPod(ns.Name, pod.Name, "c0", internal.TriliovaultLogDir, dest)
				Expect(err).To(BeNil()) // Should return nil when directory is empty/not found

				// Verify no files were created in destination
				files, err := os.ReadDir(dest)
				Expect(err).To(BeNil())
				Expect(len(files)).To(Equal(0)) // No files should be created
			})
		})
	})
})
