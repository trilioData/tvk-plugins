package cmd

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	log "github.com/sirupsen/logrus"
)

var (
	currentDir, _ = os.Getwd()
	projectRoot   = filepath.Dir(filepath.Dir(filepath.Dir(currentDir)))
	testDataDir   = filepath.Join(projectRoot, "cmd", "log-collector", "cmd", "testFiles")

	testInputFile          = "testInput.yaml"
	testInputFileDuplicate = "testInputDuplicate.yaml"
	invalidTestInputFile   = "invalidInput.yaml"
)

func TestCmd(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Log collector flag test suite")
}

var _ = BeforeSuite(func() {
	log.Info("Before Suite")
})

var _ = AfterSuite(func() {
	log.Info("After Suite")
})
