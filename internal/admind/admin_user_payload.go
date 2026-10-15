package admind

type adminUserMutation struct {
	MemberID               string   `json:"memberID,omitempty"`
	Handle                 string   `json:"handle,omitempty"`
	Name                   string   `json:"name,omitempty"`
	Email                  string   `json:"email"`
	HireDate               string   `json:"hireDate,omitempty"`
	Note                   string   `json:"note,omitempty"`
	Role                   string   `json:"role"`
	Circles                []string `json:"circles,omitempty"`
	JobTitle               string   `json:"jobTitle,omitempty"`
	GroupID                string   `json:"groupID,omitempty"`
	PhoneNumber            string   `json:"phoneNumber,omitempty"`
	SupervisorID           string   `json:"supervisorID,omitempty"`
	Status                 string   `json:"status,omitempty"`
	TemporaryPassword      string   `json:"temporaryPassword,omitempty"`
	TemporaryPasswordEmail string   `json:"temporaryPasswordEmail,omitempty"`
}
