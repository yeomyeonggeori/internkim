package admind

type pagesUsersResponse struct {
	Records          []adminUserMutation `json:"records"`
	AvailableCircles []adminCircleRecord `json:"availableCircles,omitempty"`
	AvailableGroups  []orgGroupRecord    `json:"availableGroups,omitempty"`
}
