package yml

import (
	"encoding/json"
	"fmt"
	"strings"

	"sigs.k8s.io/yaml"
)

// Metadata metadata for a kubernetes resource.
type Metadata struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

// Descriptor a descriptor for a kubernetes resource.
type Descriptor struct {
	Kind       string   `json:"kind"`
	Group      string   `json:"group"`
	APIVersion string   `json:"apiVersion"`
	Metadata   Metadata `json:"metadata"`
}

// Part is a single-part yaml source, along with its descriptor.
type Part struct {
	Contents   string
	Descriptor Descriptor
}

// Parse parses the given multi-part yaml text, and returns as Parts.
func Parse(yamlText string) ([]Part, error) {
	splitContent := SplitString(yamlText)
	parts := make([]Part, 0, len(splitContent))
	for _, part := range splitContent {
		if len(part) > 0 {
			descriptor, err := ParseDescriptor(part)
			if err != nil {
				return nil, err
			}

			if descriptor.Kind == "" && descriptor.Metadata.Name == "" {
				continue
			}

			parts = append(parts, Part{
				Contents:   part,
				Descriptor: descriptor,
			})
		}
	}
	return parts, nil
}

// ParseDescriptor parses the given single-part yaml and generates the descriptor.
func ParseDescriptor(yamlText string) (Descriptor, error) {
	d := Descriptor{}
	jsonText, err := yaml.YAMLToJSON([]byte(yamlText))
	if err != nil {
		return Descriptor{}, fmt.Errorf("failed converting YAML to JSON: %v", err)
	}

	if err := json.Unmarshal(jsonText, &d); err != nil {
		return Descriptor{}, fmt.Errorf("failed parsing descriptor: %v", err)
	}

	parts := strings.Split(d.APIVersion, "/")
	switch len(parts) {
	case 1:
		d.APIVersion = parts[0]
	case 2:
		d.Group = parts[0]
		d.APIVersion = parts[1]
	default:
		return Descriptor{}, fmt.Errorf("unexpected apiGroup: %q", d.APIVersion)
	}

	return d, nil
}
