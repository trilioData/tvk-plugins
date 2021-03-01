package api

import (
	"fmt"
	"strings"

	v1 "k8s.io/api/authorization/v1"
)

// ToSelfSubjectAccessReview creates kubernetes API object based on provided data.
func ToSelfSubjectAccessReview(namespace, name, resourceKind, verb string) *v1.SelfSubjectAccessReview {
	return &v1.SelfSubjectAccessReview{
		Spec: v1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &v1.ResourceAttributes{
				Namespace: namespace,
				Name:      name,
				// Resource kind name must be in a plural lower-case format,
				Resource: fmt.Sprintf("%ss", strings.ToLower(resourceKind)),
				// Let's enforce the same lower-case format for verbs
				Verb: strings.ToLower(verb),
			},
		},
	}
}
