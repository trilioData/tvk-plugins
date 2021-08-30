package cmd

import "k8s.io/apimachinery/pkg/util/sets"

func removeDuplicates(uids []string) []string {
	uniqueUIDs := sets.String{}
	for _, uid := range uids {
		if !uniqueUIDs.Has(uid) {
			uniqueUIDs.Insert(uid)
		}
	}

	return uniqueUIDs.List()
}
