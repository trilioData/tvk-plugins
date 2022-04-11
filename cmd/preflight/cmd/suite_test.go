package cmd

import (
	"io/fs"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
)

const (
	invalidPrefix            = "/*?/"
	invalidShortPreflightUID = "abcd"
	invalidLongPreflightUID  = "abcdefgh"

	testLogFilePrefix = "test-log"
	validPreflightUID = "abcdef"
	nodeSelKeyClass   = "class"
	nodeSelValueGold  = "gold"
	nodeSelKeyDisk    = "disk"
	nodeSelValueSSD   = "ssd"

	imagePullSecretStr = "image-pull-secret"
)

var (
	currentDir, _ = os.Getwd()
	projectRoot   = filepath.Dir(filepath.Dir(filepath.Dir(currentDir)))
	testDataDir   = filepath.Join(projectRoot, "cmd", "preflight", "cmd", "test_files")

	testInputFile        = "test_input.yaml"
	invalidTestInputFile = "invalid_input.yaml"
)

func TestCmd(t *testing.T) {
	RegisterFailHandler(Fail)
	junitReporter := reporters.NewJUnitReporter("junit-preflight-cmd-unit-tests.xml")
	RunSpecsWithDefaultAndCustomReporters(t,
		"Preflight Cmd Suite",
		[]Reporter{junitReporter})
}

var _ = BeforeSuite(func() {})

var _ = AfterSuite(func() {
	cleanDirForFiles(testLogFilePrefix)
})

// Deletes all the log files generated at the end of suite
func cleanDirForFiles(filePrefix string) {
	var names []fs.FileInfo
	names, err = ioutil.ReadDir(".")
	Expect(err).To(BeNil())
	for _, entry := range names {
		if strings.HasPrefix(entry.Name(), filePrefix) {
			err = os.RemoveAll(path.Join([]string{".", entry.Name()}...))
			Expect(err).To(BeNil())
		}
	}
}
