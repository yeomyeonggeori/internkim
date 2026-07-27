package admind

const (
	organizationEmploymentStatusActive   = "active"
	organizationEmploymentStatusLeave    = "leave"
	organizationEmploymentStatusResigned = "resigned"
)

type organizationProfile struct {
	UserID                string   `json:"userID,omitempty"`
	Email                 string   `json:"email,omitempty"`
	JobTitle              string   `json:"jobTitle,omitempty"`
	PositionLevel         int      `json:"positionLevel,omitempty"`
	GroupID               string   `json:"groupID,omitempty"`
	PhoneNumber           string   `json:"phoneNumber,omitempty"`
	HireDate              string   `json:"hireDate,omitempty"`
	SupervisorID          string   `json:"supervisorID,omitempty"`
	ProjectIDs            []string `json:"projectIDs,omitempty"`
	TeamRole              string   `json:"teamRole,omitempty"`
	EmploymentStatus      string   `json:"employmentStatus,omitempty"`
	IsOrganizationVisible bool     `json:"isOrganizationVisible"`
}

type orgGroupRecord struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ParentID string `json:"parentID,omitempty"`
}
