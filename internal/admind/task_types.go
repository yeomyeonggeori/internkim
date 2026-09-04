package admind

type taskMember struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Email              string `json:"email"`
	MattermostUsername string `json:"mattermostUsername,omitempty"`
	HireDate           string `json:"hireDate,omitempty"`
	Role               string `json:"role"`
	MattermostStatus   string `json:"mattermostStatus"`
}

type Task struct {
	ID               string   `json:"id"`
	OwnerID          string   `json:"ownerID"`
	OwnerName        string   `json:"ownerName"`
	ParticipantIDs   []string `json:"participantIDs"`
	ParticipantNames []string `json:"participantNames"`
	Business         string   `json:"business"`
	Type             string   `json:"type"`
	Content          string   `json:"content"`
	Size             string   `json:"size"`
	Status           string   `json:"status"`
	StartDate        string   `json:"startDate,omitempty"`
	EndDate          string   `json:"endDate,omitempty"`
	WeekCode         string   `json:"weekCode"`
	MattermostPostID string   `json:"mattermostPostID,omitempty"`
	CalendarEventID  string   `json:"calendarEventID,omitempty"`
	CreatedAt        string   `json:"createdAt,omitempty"`
}

type taskDefinitions struct {
	Categories     []string             `json:"categories"`
	CategoryColors map[string]string    `json:"categoryColors"`
	Types          []string             `json:"types"`
	TypeColors     map[string]string    `json:"typeColors"`
	Sizes          []taskSizeDefinition `json:"sizes"`
}

type taskSizeDefinition struct {
	Name               string `json:"name"`
	DistanceKM         int    `json:"distanceKm"`
	MaxHours           int    `json:"maxHours"`
	DevelopmentExample string `json:"developmentExample"`
	OtherExample       string `json:"otherExample"`
	Note               string `json:"note"`
	Score              int    `json:"score"`
	Label              string `json:"label"`
}

type taskWriteRequest struct {
	OwnerID         string   `json:"ownerID"`
	ParticipantIDs  []string `json:"participantIDs"`
	Business        string   `json:"business"`
	Category        string   `json:"category"`
	Type            string   `json:"type"`
	Content         string   `json:"content"`
	Goal            string   `json:"goal"`
	Size            string   `json:"size"`
	Status          string   `json:"status"`
	StartDate       string   `json:"startDate"`
	EndDate         string   `json:"endDate"`
	WeekCode        string   `json:"weekCode"`
	IsCalendarEvent bool     `json:"isCalendarEvent"`
	EventStartISO   string   `json:"eventStartISO"`
	EventEndISO     string   `json:"eventEndISO"`
	EventLocation   string   `json:"eventLocation"`
	EventAllDay     bool     `json:"eventAllDay"`
}

type taskQuickTaskRequest struct {
	Prompt         string   `json:"prompt"`
	Title          string   `json:"title"`
	EndDate        string   `json:"endDate"`
	OwnerID        string   `json:"ownerID"`
	ParticipantIDs []string `json:"participantIDs"`
	WeekCode       string   `json:"weekCode"`
	RequesterEmail string   `json:"requesterEmail"`
	Source         string   `json:"source"`
	AllowDuplicate bool     `json:"allowDuplicate"`
}

type inferredTask struct {
	Category       string   `json:"category"`
	Type           string   `json:"type"`
	Content        string   `json:"content"`
	Goal           string   `json:"goal"`
	Size           string   `json:"size"`
	Status         string   `json:"status"`
	StartDate      string   `json:"startDate"`
	EndDate        string   `json:"endDate"`
	ParticipantIDs []string `json:"participantIDs"`
}

type taskDuplicateDecision struct {
	IsDuplicate     bool   `json:"isDuplicate"`
	DuplicateTaskID string `json:"duplicateTaskID"`
	Reason          string `json:"reason"`
}

type capabilityLLMResponse struct {
	Content string `json:"content"`
}

type taskValidationError string

func (errorValue taskValidationError) Error() string {
	return string(errorValue)
}
