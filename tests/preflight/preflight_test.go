package preflighttest

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var dummy = false
var _ = Describe("Preflight Tests", func() {
	Context("test-cases", func() {
		BeforeEach(func() {
			dummy = true
		})
		Context("test-cases with dummies", func() {
			It("Dummy Run", func() {
				dummy = dummy && true
				Expect(dummy).To(Equal(true))
			})
		})
	})
})
