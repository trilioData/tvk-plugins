package common

import (
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/trilioData/k8s-triliovault/internal"
)

func IsLabelMatch(selectorList []metav1.LabelSelector, key, value string) bool {
	for index := range selectorList {
		selector := selectorList[index]
		if len(selector.MatchLabels) == 1 && len(selector.MatchExpressions) == 0 &&
			strings.EqualFold(selector.MatchLabels[key], value) {
			return true
		} else if len(selector.MatchLabels) == 0 && len(selector.MatchExpressions) == 1 &&
			IsMatchExactExpression(selector.MatchExpressions[0], key, value) {
			return true
		} else if len(selector.MatchLabels) == 1 && len(selector.MatchExpressions) == 1 &&
			strings.EqualFold(selector.MatchLabels[key], value) &&
			IsMatchExactExpression(selector.MatchExpressions[0], key, value) {
			return true
		}
	}

	return false
}

func IsMatchExactExpression(expression metav1.LabelSelectorRequirement, key, value string) bool {
	if strings.EqualFold(expression.Key, key) && expression.Operator == metav1.LabelSelectorOpIn &&
		expression.Values != nil && len(expression.Values) == 1 && strings.EqualFold(expression.Values[0], value) {
		return true
	}

	return false
}

func CheckLabelMatches(selectorList []metav1.LabelSelector, label map[string]string) bool {
	for index := range selectorList {
		selector := selectorList[index]
		labelSelector, err := metav1.LabelSelectorAsSelector(&selector)
		if err != nil {
			continue
		}
		if labelSelector.Matches(labels.Set(label)) {
			return true
		}
	}
	return false
}

func GetStringSet(str string) sets.String {
	set := sets.NewString()
	strList := strings.Split(str, internal.Comma)
	for i := range strList {
		trStr := strings.TrimSpace(strList[i])
		if len(trStr) > 0 {
			set.Insert(trStr)
		}
	}

	return set
}

func FilterNamespaceByScope(namespaceList *corev1.NamespaceList) []corev1.Namespace {
	var nsList []corev1.Namespace
	if internal.GetAppScope() == internal.NamespacedScope {
		// For namespace scope only install namespace is backup namespace
		installNs := internal.GetInstallNamespace()
		for index := range namespaceList.Items {
			ns := namespaceList.Items[index]
			if installNs == ns.Name {
				nsList = []corev1.Namespace{ns}
				break
			}
		}
	} else {
		nsList = namespaceList.Items
	}

	return nsList
}

func IsNamespacedNameExists(namespacedNames []NamespacedName, namespacedName types.NamespacedName) bool {
	for index := range namespacedNames {
		if namespacedNames[index].Namespace == namespacedName.Namespace &&
			namespacedNames[index].Name == namespacedName.Name {
			return true
		}
	}
	return false
}
