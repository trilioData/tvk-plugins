package internal

import "k8s.io/apimachinery/pkg/util/sets"

var (
	AllowedOutputFormats = sets.NewString(FormatJSON, FormatYAML, FormatWIDE)
)
