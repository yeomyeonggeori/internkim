package admind

import "net/http"

func shouldIncludeBlueclawPolicy(request *http.Request) bool {
	return request.URL.Query().Get("includePolicy") == "true"
}

type pagesUsersResponse struct {
	Records          []adminUserMutation `json:"records"`
	AvailableCircles []adminCircleRecord `json:"availableCircles,omitempty"`
	AvailableGroups  []orgGroupRecord    `json:"availableGroups,omitempty"`
}
