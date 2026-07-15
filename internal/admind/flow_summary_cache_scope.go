package admind

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

type flowSummaryMemberIdentity struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func flowSummaryDependencyKeysForWeek(weekCode string, weekStart time.Time) flowSummaryDependencyKeys {
	currentMonthStart := startOfMonth(weekStart)
	return flowSummaryDependencyKeys{
		RequestedWeek: flowSummarySourceKey{Kind: flowSummarySourceWeek, Key: weekCode},
		PreviousWeek:  flowSummarySourceKey{Kind: flowSummarySourceWeek, Key: weekCodeForDate(weekStart.AddDate(0, 0, -7))},
		CurrentMonth:  flowSummarySourceKey{Kind: flowSummarySourceMonth, Key: currentMonthStart.Format("2006-01")},
		PreviousMonth: flowSummarySourceKey{Kind: flowSummarySourceMonth, Key: currentMonthStart.AddDate(0, -1, 0).Format("2006-01")},
		Definitions:   flowSummarySourceKey{Kind: flowSummarySourceDefinitions, Key: "global"},
	}
}

func flowSummaryMemberFingerprint(members []flowMember) (string, error) {
	identities := make([]flowSummaryMemberIdentity, 0, len(members))
	for _, member := range members {
		identities = append(identities, flowSummaryMemberIdentity{ID: member.ID, Name: member.Name})
	}
	sort.Slice(identities, func(leftIndex int, rightIndex int) bool {
		if identities[leftIndex].ID != identities[rightIndex].ID {
			return identities[leftIndex].ID < identities[rightIndex].ID
		}
		return identities[leftIndex].Name < identities[rightIndex].Name
	})
	document, errorValue := json.Marshal(identities)
	if errorValue != nil {
		return "", fmt.Errorf("encode flow summary member fingerprint: %w", errorValue)
	}
	digest := sha256.Sum256(document)
	return hex.EncodeToString(digest[:]), nil
}
