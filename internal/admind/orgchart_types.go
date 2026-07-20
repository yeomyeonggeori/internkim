package admind

const (
	orgchartEmploymentStatusActive   = "active"
	orgchartEmploymentStatusLeave    = "leave"
	orgchartEmploymentStatusResigned = "resigned"
)

type orgchartProfile struct {
	UserID            string   `json:"userID,omitempty"`
	Email             string   `json:"email,omitempty"`
	JobTitle          string   `json:"jobTitle,omitempty"`
	PositionLevel     int      `json:"positionLevel,omitempty"`
	PrimaryGroupID    string   `json:"primaryGroupID,omitempty"`
	GroupIDs          []string `json:"groupIDs,omitempty"`
	SupervisorID      string   `json:"supervisorID,omitempty"`
	ProjectIDs        []string `json:"projectIDs,omitempty"`
	TeamRole          string   `json:"teamRole,omitempty"`
	EmploymentStatus  string   `json:"employmentStatus,omitempty"`
	IsOrgchartVisible bool     `json:"isOrgchartVisible"`
}

type orgGroupRecord struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ParentID string `json:"parentID,omitempty"`
}
