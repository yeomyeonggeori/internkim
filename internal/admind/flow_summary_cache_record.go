package admind

import "time"

const flowSummaryCacheSchemaVersion = 1
const flowSummaryCacheRetention = 90 * 24 * time.Hour

type flowSummarySourceKind string

const (
	flowSummarySourceWeek        flowSummarySourceKind = "week"
	flowSummarySourceMonth       flowSummarySourceKind = "month"
	flowSummarySourceDefinitions flowSummarySourceKind = "definitions"
)

type flowSummarySourceKey struct {
	Kind flowSummarySourceKind
	Key  string
}

type flowSummaryDependencyKeys struct {
	RequestedWeek flowSummarySourceKey
	PreviousWeek  flowSummarySourceKey
	CurrentMonth  flowSummarySourceKey
	PreviousMonth flowSummarySourceKey
	Definitions   flowSummarySourceKey
}

type flowSummaryDependencySnapshot struct {
	RequestedWeekRevision int64
	PreviousWeekRevision  int64
	CurrentMonthRevision  int64
	PreviousMonthRevision int64
	DefinitionsRevision   int64
	MemberFingerprint     string
}

type flowSummaryCacheEntry struct {
	WeekCode      string
	Dependencies  flowSummaryDependencySnapshot
	SchemaVersion int
	Payload       string
	CachedAt      string
}

type flowSummaryCachePayload struct {
	Version     int         `json:"version"`
	WeeklyTasks []flowTask  `json:"weeklyTasks"`
	Metrics     flowMetrics `json:"metrics"`
	Report      flowReport  `json:"report"`
}
