package cmd


import (
	"regexp"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	EndTime   = "23:59:59"
	StartTime = "00:00:00"


)

func removeDuplicates(uids []string) []string {
	uniqueUIDs := sets.String{}
	for _, uid := range uids {
		if !uniqueUIDs.Has(uid) {
			uniqueUIDs.Insert(uid)
		}
	}

	return uniqueUIDs.List()

}

func validateRFC3339Timestamps(startTimestamp string) bool {
	if _, err := time.Parse(time.RFC3339, startTimestamp); err != nil {
		return false
	}
	return true
}

// extractDate func extract date in format yyyy-mm-dd
func extractDate(timestamp string) string {

	var re = regexp.MustCompile(`([12]\d{3}-(0[1-9]|1[012])-(0[1-9]|[12]\d|3[01]))`)

	return re.FindString(timestamp)
}

// extractTime func extract time in format hh:mm:ss
func extractTime(timestamp, deltaTime string) string {
	var re = regexp.MustCompile(`([01]?\d|2[0-3]):[0-5]\d:[0-5]\d`)
	if re.FindString(timestamp) == "" {
		return deltaTime
	}
	return re.FindString(timestamp)
}

func parseTimestamp(timestamp, deltaTime string) string {
	if validateRFC3339Timestamps(timestamp) {
		return timestamp
	}
	return extractDate(timestamp) + "T" + extractTime(timestamp, deltaTime) + "Z"

}
