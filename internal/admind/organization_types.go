package admind

const (
	memberStatusActive    = "active"
	memberStatusDeparted  = "departed"
	memberStatusWithdrawn = "withdrawn"
)

type organizationProfile struct {
	MemberID     string `json:"memberID,omitempty"`
	Email        string `json:"email,omitempty"`
	JobTitle     string `json:"jobTitle,omitempty"`
	GroupID      string `json:"groupID,omitempty"`
	PhoneNumber  string `json:"phoneNumber,omitempty"`
	HireDate     string `json:"hireDate,omitempty"`
	SupervisorID string `json:"supervisorID,omitempty"`
	Status       string `json:"status,omitempty"`
}

type orgGroupRecord struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ParentID string `json:"parentID,omitempty"`
}
