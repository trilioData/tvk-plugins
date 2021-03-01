package errors

import (
	authorizationv1 "k8s.io/api/authorization/v1"

	"github.com/trilioData/k8s-triliovault/internal"
)

var trilioResources = []string{"licenses", "policies", "targets", "backups", "backupplans", "restores", "hooks"}

func HasMinimumAuthPermissions(ssrv *authorizationv1.SelfSubjectRulesReview) bool {

	// TODO: Check for other permission for listing helm charts and operators
	// Permission to list, get all trilio resources
	var trilioResourcesReadPermission, policyWritePermission bool
	for index := range ssrv.Status.ResourceRules {
		rule := ssrv.Status.ResourceRules[index]
		if sliceContains(rule.APIGroups, internal.Star) ||
			sliceContains(rule.APIGroups, internal.TrilioVaultGroup) {
			if sliceContains(rule.Resources, internal.Star) || sliceContainsAll(rule.Resources, trilioResources) {
				if sliceContains(rule.Verbs, internal.Star) ||
					sliceContainsAll(rule.Verbs, []string{internal.GET, internal.LIST}) {
					trilioResourcesReadPermission = true
				}
			}
			if sliceContains(rule.Resources, internal.Star) || sliceContains(rule.Resources, "policies") {
				if sliceContains(rule.Verbs, internal.Star) || sliceContains(rule.Verbs, internal.CREATE) {
					policyWritePermission = true
				}
			}
		}
	}

	if trilioResourcesReadPermission && policyWritePermission {
		return true
	}

	return false
}

func sliceContains(strings []string, str string) bool {
	for _, v := range strings {
		if v == str {
			return true
		}
	}

	return false
}

func sliceContainsAll(strings1, strings2 []string) bool {
	strings1Map := make(map[string]bool)
	for _, v := range strings1 {
		strings1Map[v] = true
	}
	for _, v := range strings2 {
		if _, ok := strings1Map[v]; !ok {
			return false
		}
	}

	return true
}
