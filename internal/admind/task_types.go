package admind

type taskSummaryResponse struct {
	Week        taskWeek    `json:"week"`
	CurrentWeek taskWeek    `json:"currentWeek"`
	WeeklyTasks []Task      `json:"weeklyTasks"`
	Metrics     taskMetrics `json:"metrics"`
	Report      taskReport  `json:"report"`
	Source      string      `json:"source"`
}

type taskStateResponse struct {
	CurrentWeek      taskWeek        `json:"currentWeek"`
	Members          []taskMember    `json:"members"`
	Tasks            []Task          `json:"tasks"`
	Metrics          taskMetrics     `json:"metrics"`
	Definitions      taskDefinitions `json:"definitions"`
	StatusOptions    []string        `json:"statusOptions"`
	CurrentUserEmail string          `json:"currentUserEmail"`
	CurrentUserName  string          `json:"currentUserName"`
	IsAdmin          bool            `json:"isAdmin"`
	Source           string          `json:"source"`
}

type taskStatusResponse struct {
	DatabasePath string `json:"databasePath"`
	Exists       bool   `json:"exists"`
	Ready        bool   `json:"ready"`
	Message      string `json:"message"`
}

type taskWeek struct {
	Code      string `json:"code"`
	StartISO  string `json:"startISO"`
	EndISO    string `json:"endISO"`
	Previous  string `json:"previous"`
	Next      string `json:"next"`
	IsCurrent bool   `json:"isCurrent"`
}

type taskMember struct {
	ID                 string `json:"id"`
	Name               string `json:"name"`
	Email              string `json:"email"`
	Image              string `json:"image,omitempty"`
	MattermostUsername string `json:"mattermostUsername,omitempty"`
	HireDate           string `json:"hireDate,omitempty"`
	Role               string `json:"role"`
	MattermostStatus   string `json:"mattermostStatus"`
	Distance           int    `json:"distance"`
	Score              int    `json:"score"`
	ActiveTaskCount    int    `json:"activeTaskCount"`
	CompleteTaskCount  int    `json:"completeTaskCount"`
}

type Task struct {
	ID                 string   `json:"id"`
	OwnerID            string   `json:"ownerID"`
	OwnerName          string   `json:"ownerName"`
	ParticipantIDs     []string `json:"participantIDs"`
	ParticipantNames   []string `json:"participantNames"`
	Business           string   `json:"business"`
	Type               string   `json:"type"`
	Content            string   `json:"content"`
	Size               string   `json:"size"`
	Status             string   `json:"status"`
	StatusRank         int      `json:"statusRank"`
	StatusRankProvided bool     `json:"-"`
	StartDate          string   `json:"startDate,omitempty"`
	EndDate            string   `json:"endDate,omitempty"`
	WeekCode           string   `json:"weekCode"`
	MattermostPostID   string   `json:"mattermostPostID,omitempty"`
	CalendarEventID    string   `json:"calendarEventID,omitempty"`
	CreatedAt          string   `json:"createdAt,omitempty"`
}

type taskMetrics struct {
	TotalTasks         int                            `json:"totalTasks"`
	CompletedTasks     int                            `json:"completedTasks"`
	RequestedTasks     int                            `json:"requestedTasks"`
	PausedTasks        int                            `json:"pausedTasks"`
	StoppedTasks       int                            `json:"stoppedTasks"`
	TotalDistance      int                            `json:"totalDistance"`
	TotalScore         int                            `json:"totalScore"`
	StatusCounts       map[string]int                 `json:"statusCounts"`
	BusinessCounts     map[string]int                 `json:"businessCounts"`
	TypeCounts         map[string]int                 `json:"typeCounts"`
	MemberDistances    map[string]int                 `json:"memberDistances"`
	MemberScores       map[string]int                 `json:"memberScores"`
	MemberScoreDetails map[string]taskMemberScoreItem `json:"memberScoreDetails"`
}

type taskMemberScoreItem struct {
	WeeklyScore  int `json:"weeklyScore"`
	MonthlyScore int `json:"monthlyScore"`
	CurrentScore int `json:"currentScore"`
}

type taskReport struct {
	WeeklyDistanceTrend  taskDistanceTrend `json:"weeklyDistanceTrend"`
	MonthlyDistanceTrend taskDistanceTrend `json:"monthlyDistanceTrend"`
}

type taskDistanceTrend struct {
	Labels         []string `json:"labels"`
	CurrentLabel   string   `json:"currentLabel"`
	PreviousLabel  string   `json:"previousLabel"`
	CurrentValues  []int    `json:"currentValues"`
	PreviousValues []int    `json:"previousValues"`
	CurrentTotal   int      `json:"currentTotal"`
	PreviousTotal  int      `json:"previousTotal"`
	Unit           string   `json:"unit"`
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
	StatusRank      *int     `json:"statusRank"`
	StartDate       string   `json:"startDate"`
	EndDate         string   `json:"endDate"`
	WeekCode        string   `json:"weekCode"`
	Flag            int      `json:"flag"`
	RequestReason   string   `json:"requestReason"`
	DecisionReason  string   `json:"decisionReason"`
	IsCalendarEvent bool     `json:"isCalendarEvent"`
	EventStartISO   string   `json:"eventStartISO"`
	EventEndISO     string   `json:"eventEndISO"`
	EventLocation   string   `json:"eventLocation"`
	EventAllDay     bool     `json:"eventAllDay"`
}

type taskBoardMoveRequest struct {
	TaskID       string  `json:"taskID"`
	TargetStatus string  `json:"targetStatus"`
	BeforeTaskID *string `json:"beforeTaskID"`
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

type taskDefinitionsWriteRequest struct {
	Categories     []string             `json:"categories"`
	CategoryColors map[string]string    `json:"categoryColors"`
	Types          []string             `json:"types"`
	TypeColors     map[string]string    `json:"typeColors"`
	Sizes          []taskSizeDefinition `json:"sizes"`
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
	RequestReason  string   `json:"requestReason"`
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
