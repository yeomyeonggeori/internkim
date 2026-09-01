package admind

import (
	"net/url"
	"strings"
	"time"
	"unicode"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

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

func taskNotifyAddressByPersonID(records []adminUserMutation) map[string]string {
	addressByPersonID := map[string]string{}
	for _, record := range records {
		email := strings.ToLower(strings.TrimSpace(record.Email))
		if record.MemberID == "" || email == "" {
			continue
		}
		addressByPersonID[record.MemberID] = email
	}
	return addressByPersonID
}
