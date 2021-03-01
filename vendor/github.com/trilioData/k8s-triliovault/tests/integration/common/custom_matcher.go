package common

import (
	"reflect"

	"github.com/onsi/gomega/format"
	"github.com/onsi/gomega/types"
)

func isNotNil(a interface{}) bool {
	if a != nil {
		return true
	}

	switch reflect.TypeOf(a).Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return !reflect.ValueOf(a).IsNil()
	}

	return true
}

type BeNotNilMatcher struct {
}

func (matcher *BeNotNilMatcher) Match(actual interface{}) (success bool, err error) {
	return isNotNil(actual), nil
}

func (matcher *BeNotNilMatcher) FailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "to be not nil")
}

func (matcher *BeNotNilMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	return format.Message(actual, "to be nil")
}

// BeNotNil succeeds if actual is not nil
func BeNotNil() types.GomegaMatcher {
	return &BeNotNilMatcher{}
}
