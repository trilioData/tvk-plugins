package main_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Add:", func() {
	Context("when summand is negative", func() {
		It("returns an err", func() {
			_, err := Add(-1, -1)
			Expect(err).To(HaveOccurred())
		})
	})
})

var ErrInvalidSummand = errors.New("invalid summand")

func Add(x, y int) (int, error) {
	if x <= 0 || y <= 0 {
		return 0, ErrInvalidSummand
	}
	return x + y, nil
}
