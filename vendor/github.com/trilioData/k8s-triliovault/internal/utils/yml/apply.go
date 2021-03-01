package yml

import (
	"fmt"
	"reflect"

	"sigs.k8s.io/yaml"
)

// ApplyNamespace applies the given namespaces to the resources in the yamlText.
func ApplyNamespace(yamlText, ns string) (string, error) {
	chunks := SplitString(yamlText)

	toJoin := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		var err error
		chunk, err = applyNamespace(chunk, ns)
		if err != nil {
			return "", err
		}
		toJoin = append(toJoin, chunk)
	}

	result := JoinString(toJoin...)
	return result, nil
}

func applyNamespace(yamlText, ns string) (string, error) {
	m := make(map[string]interface{})
	if err := yaml.Unmarshal([]byte(yamlText), &m); err != nil {
		return "", err
	}

	meta, err := ensureChildMap(m, "metadata")
	if err != nil {
		return "", err
	}
	meta["namespace"] = ns

	by, err := yaml.Marshal(m)
	if err != nil {
		return "", err
	}

	return string(by), nil
}

func ensureChildMap(m map[string]interface{}, name string) (map[string]interface{}, error) {
	c, ok := m[name]
	if !ok {
		c = make(map[string]interface{})
	}

	cm, ok := c.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("child %q field is not a map: %v", name, reflect.TypeOf(c))
	}

	return cm, nil
}
