package cmd

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
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
	junitReporter := reporters.NewJUnitReporter("junit-log-collector-cmd-unit-tests.xml")
	RunSpecsWithDefaultAndCustomReporters(t,
		"Log Collector Cmd Suite",
		[]Reporter{junitReporter})
}

var _ = BeforeSuite(func() {
	log.Info("Before Suite")
})

var _ = AfterSuite(func() {
	log.Info("After Suite")
})
