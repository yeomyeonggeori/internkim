package admind

type calendarParticipantIdentity struct {
	PersonID string `json:"personID"`
	Name     string `json:"name"`
	Email    string `json:"email,omitempty"`
}

type calendarParticipant struct {
	PersonID string `json:"personID"`
	Name     string `json:"name"`
	Email    string `json:"email,omitempty"`
	Image    string `json:"image,omitempty"`
}
