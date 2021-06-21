package target_browser_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/onsi/ginkgo/reporters"
)

func TestTargetBrowser(t *testing.T) {
	RegisterFailHandler(Fail)
	junitReporter := reporters.NewJUnitReporter("target-browser-junit.xml")
	RunSpecsWithCustomReporters(t, "TargetBrowser Suite", []Reporter{junitReporter})
}
