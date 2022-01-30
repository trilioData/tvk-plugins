package preflighttest

import (
	"testing"

	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
)

func TestPreflight(t *testing.T) {
	RegisterFailHandler(Fail)
	junitReporter := reporters.NewJUnitReporter("preflight-junit.xml")
	RunSpecsWithDefaultAndCustomReporters(t, "preflight Suite", []Reporter{junitReporter})
}
