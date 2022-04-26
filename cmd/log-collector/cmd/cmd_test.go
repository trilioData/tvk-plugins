package cmd

import (
	"io/ioutil"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
	"sigs.k8s.io/yaml"

	logcollector "github.com/trilioData/tvk-plugins/tools/log-collector"
)

var _ = Describe("log collector cmd helper unit tests", func() {

	Context("Initialization of log collector", func() {

		It("Should initialize log collector objects when valid kubeconfig file path is provided", func() {

			command := logCollectorCommand()
			command.PersistentPreRunE(command, []string{})
			Expect(logCollector.KubeConfig).ShouldNot(BeEmpty())
			Expect(logCollector.Clustered).Should(BeFalse())
			Expect(logCollector.Namespaces).ShouldNot(BeEmpty())
			Expect(inputFileName).Should(BeEmpty())
			Expect(logCollector.Namespaces[0]).Should(Equal(defaultNamespace))
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
			command.PersistentPreRunE(command, []string{})
			log.Info(logCollector)

			data, err := ioutil.ReadFile(inputFileName)
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

			data, err := ioutil.ReadFile(inputFileName)
			Expect(err).ShouldNot(HaveOccurred())
			newLCerr := yaml.UnmarshalStrict(data, &newLogCollector)
			Expect(pErr.Error()).Should(Equal(newLCerr.Error()))
		})

		It("Should filter duplicate values from gvk and label selector", func() {

			var newLogCollector logcollector.LogCollector
			command := logCollectorCommand()
			inputFileName = filepath.Join(testDataDir, testInputFileDuplicate)
			command.PersistentPreRunE(command, []string{})
			log.Info(logCollector)

			data, err := ioutil.ReadFile(inputFileName)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(yaml.UnmarshalStrict(data, &newLogCollector)).ShouldNot(HaveOccurred())

			Expect(len(newLogCollector.GroupVersionKinds)).ShouldNot(Equal(len(logCollector.GroupVersionKinds)))
			Expect(len(newLogCollector.LabelSelectors)).ShouldNot(Equal(len(logCollector.LabelSelectors)))

			// check if version populates in pre run if not given
			Expect(newLogCollector.GroupVersionKinds[0].Version).Should(BeEmpty())
			Expect(logCollector.GroupVersionKinds[0].Version).ShouldNot(BeEmpty())

			Expect(newLogCollector.CleanOutput).Should(Equal(logCollector.CleanOutput))
			Expect(newLogCollector.Clustered).Should(Equal(logCollector.Clustered))
			Expect(newLogCollector.Loglevel).Should(Equal(logCollector.Loglevel))
		})

		It("Should return error when kubeconfig file contains invalid data", func() {

			logCollector.KubeConfig = "invalid/path/to/kubeconfig"
			err := logCollector.InitializeKubeClients()
			Expect(err).ShouldNot(BeNil())
		})
	})
})
