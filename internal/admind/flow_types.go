package admind

type flowSummaryResponse struct {
	Week        flowWeek    `json:"week"`
	CurrentWeek flowWeek    `json:"currentWeek"`
	WeeklyTasks []flowTask  `json:"weeklyTasks"`
	Metrics     flowMetrics `json:"metrics"`
	Report      flowReport  `json:"report"`
	Source      string      `json:"source"`
}

type flowStateResponse struct {
	CurrentWeek      flowWeek        `json:"currentWeek"`
	Members          []flowMember    `json:"members"`
	Tasks            []flowTask      `json:"tasks"`
	Metrics          flowMetrics     `json:"metrics"`
	Definitions      flowDefinitions `json:"definitions"`
	StatusOptions    []string        `json:"statusOptions"`
	CurrentUserEmail string          `json:"currentUserEmail"`
	CurrentUserName  string          `json:"currentUserName"`
	IsAdmin          bool            `json:"isAdmin"`
	Source           string          `json:"source"`
}

type flowStatusResponse struct {
	DatabasePath string `json:"databasePath"`
	Exists       bool   `json:"exists"`
	Ready        bool   `json:"ready"`
	Message      string `json:"message"`
}

type flowWeek struct {
	Code      string `json:"code"`
	StartISO  string `json:"startISO"`
	EndISO    string `json:"endISO"`
	Previous  string `json:"previous"`
	Next      string `json:"next"`
	IsCurrent bool   `json:"isCurrent"`
}

type flowMember struct {
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

type flowTask struct {
	ID                 string   `json:"id"`
	OwnerID            string   `json:"ownerID"`
	OwnerName          string   `json:"ownerName"`
	ParticipantIDs     []string `json:"participantIDs"`
	ParticipantNames   []string `json:"participantNames"`
	Business           string   `json:"business"`
	Type               string   `json:"type"`
	Content            string   `json:"content"`
	Goal               string   `json:"goal"`
	Size               string   `json:"size"`
	Status             string   `json:"status"`
	StatusRank         int      `json:"statusRank"`
	StatusRankProvided bool     `json:"-"`
	StartDate          string   `json:"startDate,omitempty"`
	EndDate            string   `json:"endDate,omitempty"`
	WeekCode           string   `json:"weekCode"`
	Flag               int      `json:"flag"`
	RequestReason      string   `json:"requestReason,omitempty"`
	DecisionReason     string   `json:"decisionReason,omitempty"`
	MattermostPostID   string   `json:"mattermostPostID,omitempty"`
	CreatedAt          string   `json:"createdAt,omitempty"`
}

type flowMetrics struct {
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
	MemberScoreDetails map[string]flowMemberScoreItem `json:"memberScoreDetails"`
}

type flowMemberScoreItem struct {
	WeeklyScore  int `json:"weeklyScore"`
	MonthlyScore int `json:"monthlyScore"`
	CurrentScore int `json:"currentScore"`
}

type flowReport struct {
	WeeklyDistanceTrend  flowDistanceTrend `json:"weeklyDistanceTrend"`
	MonthlyDistanceTrend flowDistanceTrend `json:"monthlyDistanceTrend"`
}

type flowDistanceTrend struct {
	Labels         []string `json:"labels"`
	CurrentLabel   string   `json:"currentLabel"`
	PreviousLabel  string   `json:"previousLabel"`
	CurrentValues  []int    `json:"currentValues"`
	PreviousValues []int    `json:"previousValues"`
	CurrentTotal   int      `json:"currentTotal"`
	PreviousTotal  int      `json:"previousTotal"`
	Unit           string   `json:"unit"`
}

type flowDefinitions struct {
	Categories []string             `json:"categories"`
	Types      []string             `json:"types"`
	Sizes      []flowSizeDefinition `json:"sizes"`
}

type flowSizeDefinition struct {
	Name               string `json:"name"`
	DistanceKM         int    `json:"distanceKm"`
	MaxHours           int    `json:"maxHours"`
	DevelopmentExample string `json:"developmentExample"`
	OtherExample       string `json:"otherExample"`
	Note               string `json:"note"`
	Score              int    `json:"score"`
	Label              string `json:"label"`
}

type flowTaskWriteRequest struct {
	OwnerID        string   `json:"ownerID"`
	ParticipantIDs []string `json:"participantIDs"`
	Business       string   `json:"business"`
	Category       string   `json:"category"`
	Type           string   `json:"type"`
	Content        string   `json:"content"`
	Goal           string   `json:"goal"`
	Size           string   `json:"size"`
	Status         string   `json:"status"`
	StatusRank     *int     `json:"statusRank"`
	StartDate      string   `json:"startDate"`
	EndDate        string   `json:"endDate"`
	WeekCode       string   `json:"weekCode"`
	Flag           int      `json:"flag"`
	RequestReason  string   `json:"requestReason"`
	DecisionReason string   `json:"decisionReason"`
}

type flowTaskBoardMoveRequest struct {
	TaskID       string  `json:"taskID"`
	TargetStatus string  `json:"targetStatus"`
	BeforeTaskID *string `json:"beforeTaskID"`
}

type flowQuickTaskRequest struct {
	Prompt         string   `json:"prompt"`
	OwnerID        string   `json:"ownerID"`
	ParticipantIDs []string `json:"participantIDs"`
	WeekCode       string   `json:"weekCode"`
	RequesterEmail string   `json:"requesterEmail"`
	Source         string   `json:"source"`
	AllowDuplicate bool     `json:"allowDuplicate"`
}

type flowDefinitionsWriteRequest struct {
	Categories []string             `json:"categories"`
	Types      []string             `json:"types"`
	Sizes      []flowSizeDefinition `json:"sizes"`
}

type inferredFlowTask struct {
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

type flowDuplicateDecision struct {
	IsDuplicate     bool   `json:"isDuplicate"`
	DuplicateTaskID string `json:"duplicateTaskID"`
	Reason          string `json:"reason"`
}

type capabilityLLMResponse struct {
	Content string `json:"content"`
}

type flowValidationError string

func (errorValue flowValidationError) Error() string {
	return string(errorValue)
}
