package preflight

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	"github.com/sirupsen/logrus"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	clientGoScheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
)

var (
	ctx           = context.Background()
	k8sClient     client.Client
	testEnv       = &envtest.Environment{}
	envTestScheme *runtime.Scheme
	logger        *logrus.Logger
	run           *Run
)

func TestPreflight(t *testing.T) {
	RegisterFailHandler(Fail)
	junitReporter := reporters.NewJUnitReporter("junit-preflight-unit-test.xml")
	RunSpecsWithDefaultAndCustomReporters(t,
		"webhook-server",
		[]Reporter{printer.NewlineReporter{}, junitReporter})
}

var _ = BeforeSuite(func() {

	By("Bootstrapping test environment")
	envTestScheme = runtime.NewScheme()
	Expect(apiextensionsv1.AddToScheme(envTestScheme)).To(BeNil())
	Expect(apiextensionsv1beta1.AddToScheme(envTestScheme)).To(BeNil())
	Expect(clientGoScheme.AddToScheme(envTestScheme)).To(BeNil())

	// starting the env cluster
	cfg, err := testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: envTestScheme,
	})
	Expect(err).ToNot(HaveOccurred())

	go func() {
		err = k8sManager.Start(ctx)
		Expect(err).ToNot(HaveOccurred())
	}()

	k8sClient = k8sManager.GetClient()
	Expect(k8sClient).ToNot(BeNil())
	runtimeClient = k8sClient

	logger = logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{ForceColors: true})
	run = &Run{CommonOptions: CommonOptions{Logger: logger}}
})
