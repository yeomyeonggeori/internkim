package admind

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/hostupdate"
)

const (
	hostUpdateReportInterval = 15 * time.Second
	hostUpdateLaunchGrace    = 2 * time.Minute
	hostUpdateReportDelay    = 15 * time.Second
	hostUpdateScheduleCreate = "/admin/api/schedule/tool-create"
)

type hostUpdateReportSchedule struct {
	TaskInstruction string `json:"taskInstruction"`
	Description     string `json:"description"`
	Kind            string `json:"kind"`
	RunAt           string `json:"runAt"`
	TimeZone        string `json:"timeZone"`
	Platform        string `json:"platform,omitempty"`
	ConversationID  string `json:"conversationID,omitempty"`
	ReplyTargetID   string `json:"replyTargetID,omitempty"`
}

func (service *Service) keepHostUpdateReported(ctx context.Context) {
	for {
		service.reportHostUpdateOnce(ctx)
		select {
		case <-ctx.Done():
			return
		case <-time.After(hostUpdateReportInterval):
		}
	}
}

func (service *Service) reportHostUpdateOnce(ctx context.Context) {
	dependencies := service.hostUpdate()
	note, isPending, errorValue := hostupdate.ReadNote(dependencies.NotePath)
	if errorValue != nil || !isPending || !settleAnAbandonedUpdate(dependencies, &note) || !note.Requester.HasConversation() {
		return
	}
	if errorValue := service.tellRequesterHowTheUpdateEnded(ctx, note); errorValue != nil {
		log.Printf("host update: the result to %s is not delivered yet: %v", note.Requester.Email, errorValue)
		return
	}
	if errorValue := hostupdate.ClearNote(dependencies.NotePath); errorValue != nil {
		log.Printf("host update: the delivered note could not be cleared: %v", errorValue)
	}
}

func settleAnAbandonedUpdate(dependencies hostUpdateDependencies, note *hostupdate.Note) bool {
	if note.IsFinished() {
		return true
	}
	if dependencies.Supervisor.IsRunning() || dependencies.Now().Sub(note.StartedAt) < hostUpdateLaunchGrace {
		return false
	}
	note.Outcome = &hostupdate.Outcome{FinishedAt: dependencies.Now().UTC(), Error: "the update stopped before it recorded a result, as when the machine restarts during it"}
	return hostupdate.WriteNote(dependencies.NotePath, *note) == nil
}

func (service *Service) tellRequesterHowTheUpdateEnded(ctx context.Context, note hostupdate.Note) error {
	body, errorValue := service.hostUpdateReportSchedule(ctx, note)
	if errorValue != nil {
		return errorValue
	}
	status, answer, errorValue := service.blueclawSignedAnswer(ctx, http.MethodPost, hostUpdateScheduleCreate, body, note.Requester.PersonID)
	if errorValue != nil {
		return errorValue
	}
	if status >= http.StatusInternalServerError {
		return fmt.Errorf("the agent answered %d: %s", status, strings.TrimSpace(string(answer)))
	}
	if status >= http.StatusBadRequest {
		log.Printf("host update: the agent refused to report the result to %s and never will: %d %s", note.Requester.Email, status, strings.TrimSpace(string(answer)))
	}
	return nil
}

func (service *Service) hostUpdateReportSchedule(ctx context.Context, note hostupdate.Note) ([]byte, error) {
	dependencies := service.hostUpdate()
	companyLocation := service.companyTimeLocation(ctx)
	facts, errorValue := json.Marshal(map[string]any{
		"fromVersion":         note.FromVersion,
		"toVersion":           note.ToVersion,
		"installedVersionNow": dependencies.Machine.InstalledVersion,
		"startedAt":           note.StartedAt.In(companyLocation).Format(time.RFC3339),
		"outcome":             outcomeInLocation(note.Outcome, companyLocation),
	})
	if errorValue != nil {
		return nil, errorValue
	}
	return json.Marshal(hostUpdateReportSchedule{
		TaskInstruction: "Tell the requester how the update of this company's host that they approved has ended, in one short message in this conversation. Use only these facts and invent none: " + string(facts),
		Description:     "Host update result",
		Kind:            "once",
		RunAt:           dependencies.Now().Add(hostUpdateReportDelay).UTC().Format(time.RFC3339),
		TimeZone:        companyLocation.String(),
		Platform:        note.Requester.Platform,
		ConversationID:  note.Requester.ConversationID,
		ReplyTargetID:   note.Requester.ReplyTargetID,
	})
}

func outcomeInLocation(outcome *hostupdate.Outcome, location *time.Location) *hostupdate.Outcome {
	if outcome == nil {
		return nil
	}
	inLocation := *outcome
	inLocation.FinishedAt = outcome.FinishedAt.In(location)
	return &inLocation
}
