package admind

import (
	"context"
	"log"
	"net/http"
)

// A person record written before internkim#507 names the circle everyone is in
// by its old name, so people carry both and the older one points at a directory
// the migration emptied. Normalising a record drops it, but a record nobody
// touches is never normalised, so the device is asked once at start.
func (service *Service) dropTheFormerCircleFromPeople(ctx context.Context) {
	var policyDocument map[string]any
	if errorValue := service.blueclawJSONRequest(ctx, http.MethodGet, "/admin/api/policy", nil, &policyDocument); errorValue != nil {
		return
	}
	people, _ := policyDocument["people"].([]any)
	changedCount := 0
	for _, held := range people {
		person, isPerson := held.(map[string]any)
		if !isPerson {
			continue
		}
		kept := circlesWithoutTheFormerName(policyStringList(person["circles"]))
		if len(kept) == len(policyStringList(person["circles"])) {
			continue
		}
		person["circles"] = kept
		changedCount++
	}
	if changedCount == 0 {
		return
	}
	if errorValue := service.deliverBlueclawPolicy(ctx, policyDocument); errorValue != nil {
		log.Printf("dropping the former circle name from %d people failed: %v", changedCount, errorValue)
		return
	}
	log.Printf("the former circle name is gone from %d people", changedCount)
}

func circlesWithoutTheFormerName(circles []string) []string {
	kept := make([]string, 0, len(circles))
	for _, circle := range circles {
		if circle == formerMemberCircleName {
			continue
		}
		kept = append(kept, circle)
	}
	return kept
}
