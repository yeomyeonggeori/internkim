package admind

import "time"

const taskSummaryCacheSchemaVersion = 1
const taskSummaryCacheRetention = 90 * 24 * time.Hour

type taskSummarySourceKind string

const (
	taskSummarySourceWeek        taskSummarySourceKind = "week"
	taskSummarySourceMonth       taskSummarySourceKind = "month"
	taskSummarySourceDefinitions taskSummarySourceKind = "definitions"
)

type taskSummarySourceKey struct {
	Kind taskSummarySourceKind
	Key  string
}

type taskSummaryDependencyKeys struct {
	RequestedWeek taskSummarySourceKey
	PreviousWeek  taskSummarySourceKey
	CurrentMonth  taskSummarySourceKey
	PreviousMonth taskSummarySourceKey
	Definitions   taskSummarySourceKey
}

type taskSummaryDependencySnapshot struct {
	RequestedWeekRevision int64
	PreviousWeekRevision  int64
	CurrentMonthRevision  int64
	PreviousMonthRevision int64
	DefinitionsRevision   int64
	MemberFingerprint     string
}

type taskSummaryCacheEntry struct {
	WeekCode      string
	Dependencies  taskSummaryDependencySnapshot
	SchemaVersion int
	Payload       string
}

type taskSummaryCachePayload struct {
	Version     int         `json:"version"`
	WeeklyTasks []Task  `json:"weeklyTasks"`
	Metrics     taskMetrics `json:"metrics"`
	Report      taskReport  `json:"report"`
}
