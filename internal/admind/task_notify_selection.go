package admind

import (
	"net/url"
	"strings"
	"time"
	"unicode"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

// One run as the task monitor reports it. The field names are blueclaw's, so a
// rename there fails to decode here rather than silently notifying nothing.
type taskNotifyRun struct {
	TaskRunID         string    `json:"taskRunID"`
	RequesterPersonID string    `json:"requesterPersonID"`
	Status            string    `json:"status"`
	Prompt            string    `json:"prompt"`
	Result            string    `json:"result"`
	FailureReason     string    `json:"failureReason"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

const (
	taskNotifyTitleRunes = 40
	taskNotifyBodyRunes  = 120
)

// Which transitions are worth waking a phone for. The run's own status decides
// it: the ledger has no task.failed event, and a failure is recorded as
// task.paused, so an event name is the wrong thing to watch.
//
// waiting_user_input also blocks on the requester and is deliberately left out
// until the reply path can carry an answer back.
func taskNotifyCategory(status string) (string, bool) {
	switch status {
	case "waiting_approval":
		return "approval", true
	case "completed", "failed":
		return "task", true
	default:
		return "", false
	}
}

// The runtime relays what it recorded: a status label the task board already
// uses, and the agent's own words for the body. It composes no explanation.
func taskNotifyContent(run taskNotifyRun, category string, confirmationMessage string) centralplane.Notification {
	title, body := "", ""
	switch run.Status {
	case "waiting_approval":
		title = "승인 대기"
		body = firstNonEmpty(confirmationMessage, run.Prompt)
	case "completed":
		title = "작업 완료"
		body = firstNonEmpty(run.Result, run.Prompt)
	case "failed":
		title = "작업 실패"
		body = firstNonEmpty(run.FailureReason, run.Prompt)
	}
	if subject := taskNotifyExcerpt(run.Prompt, taskNotifyTitleRunes); subject != "" {
		title = title + ": " + subject
	}
	return centralplane.Notification{
		Category: category,
		Title:    title,
		Body:     taskNotifyExcerpt(body, taskNotifyBodyRunes),
		OpenPath: "/tasks/" + url.PathEscape(run.TaskRunID),
		Tag:      "task-run-" + run.TaskRunID,
	}
}

// Counted in runes, because a notification body cut mid-character on Korean
// text arrives as a replacement glyph.
func taskNotifyExcerpt(text string, longest int) string {
	flattened := strings.Join(strings.FieldsFunc(text, func(character rune) bool {
		return character == '\n' || character == '\r' || unicode.IsControl(character)
	}), " ")
	flattened = strings.TrimSpace(flattened)
	runes := []rune(flattened)
	if len(runes) <= longest {
		return flattened
	}
	return strings.TrimSpace(string(runes[:longest])) + "…"
}

// blueclaw knows a requester by the person id this device projected, which is
// the account directory's own user id, so the join is an exact lookup.
func taskNotifyExternalIDByPersonID(records []adminUserMutation) map[string]string {
	externalIDByPersonID := map[string]string{}
	for _, record := range records {
		if record.UserID == "" || record.MattermostUserID == "" {
			continue
		}
		externalIDByPersonID[record.UserID] = record.MattermostUserID
	}
	return externalIDByPersonID
}
