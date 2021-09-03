package cmd

import (
	"fmt"
	"time"

	"github.com/araddon/dateparse"
	"k8s.io/apimachinery/pkg/util/sets"
)

const (
	EndTime           = "23:59:59"
	StartTime         = "00:00:00"
	dayEndTimeSeconds = 24*60*60 - 1
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

func parseTimestamp(timestamp, deltaTime string) (time.Time, error) {
	ts, err := dateparse.ParseAny(timestamp)
	if err != nil {
		return time.Time{}, err
	}
	if ts.Hour() == 0 && ts.Minute() == 0 && ts.Second() == 0 && deltaTime != StartTime {
		ts = ts.Add(dayEndTimeSeconds * time.Second)
	}
	return ts, nil

}

func validateStartEndTimeStamp(startTS, endTS, startFlag, endFlag, startUsage, endUsage string) (startTStamp, endTStamp string, err error) {
	var ts time.Time
	if startTS != "" && endTS != "" {
		ts, err = parseTimestamp(startTS, StartTime)
		if err != nil {
			return "", "", fmt.Errorf("[%s] flag invalid value. Usage - %s", startFlag, startUsage)
		}
		startTStamp = ts.Format(time.RFC3339)
		ts, err = parseTimestamp(endTS, EndTime)
		if err != nil {
			return "", "", fmt.Errorf("[%s] flag invalid value. Usage - %s", endFlag, endUsage)
		}
		endTStamp = ts.Format(time.RFC3339)

	} else if endTS != "" {
		ts, err = parseTimestamp(endTS, EndTime)
		if err != nil {
			return "", "", fmt.Errorf("[%s] flag invalid value. Usage - %s", endFlag, endUsage)
		}
		endTStamp = ts.Format(time.RFC3339)
		startTStamp = ts.Truncate(time.Hour * 24).Format(time.RFC3339)
	} else if startTS != "" {
		ts, err = parseTimestamp(startTS, StartTime)
		if err != nil {
			return "", "", fmt.Errorf("[%s] flag invalid value. Usage - %s", startFlag, startUsage)
		}
		startTStamp = ts.Format(time.RFC3339)
		endTStamp = ts.Truncate(time.Hour * 24).Add(dayEndTimeSeconds * time.Second).Format(time.RFC3339)
	}
	return startTStamp, endTStamp, nil
}
