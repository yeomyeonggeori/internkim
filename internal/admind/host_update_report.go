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
	hostUpdateReportInterval    = 15 * time.Second
	hostUpdateLaunchGrace       = 2 * time.Minute
	hostUpdateReportDelay       = 15 * time.Second
	hostUpdateReportDescription = "Host update result"
	hostUpdateScheduleCreate    = "/admin/api/schedule/tool-create"
)

type hostUpdateResultFacts struct {
	FromVersion      string `json:"fromVersion"`
	ToVersion        string `json:"toVersion"`
	StartedAt        string `json:"startedAt"`
	FinishedAt       string `json:"finishedAt"`
	Succeeded        bool   `json:"succeeded"`
	InstalledVersion string `json:"installedVersion"`
	Error            string `json:"error,omitempty"`
	InstallerOutput  string `json:"installerOutputTail,omitempty"`
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
	if errorValue != nil {
		log.Printf("host update: the pending note is unreadable: %v", errorValue)
		return
	}
	if !isPending {
		return
	}
	note, isSettled := settleAnAbandonedUpdate(dependencies, note)
	if !isSettled {
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

func settleAnAbandonedUpdate(dependencies hostUpdateDependencies, note hostupdate.Note) (hostupdate.Note, bool) {
	if note.IsFinished() {
		return note, true
	}
	if dependencies.Supervisor.IsRunning() || dependencies.Now().Sub(note.StartedAt) < hostUpdateLaunchGrace {
		return note, false
	}
	note.Outcome = &hostupdate.Outcome{
		FinishedAt:       dependencies.Now().UTC(),
		InstalledVersion: dependencies.Machine.InstalledVersion(),
		Error:            "the update stopped before it recorded a result, as when the machine restarts during it",
	}
	if errorValue := hostupdate.WriteNote(dependencies.NotePath, note); errorValue != nil {
		log.Printf("host update: the abandoned update could not be recorded: %v", errorValue)
		return note, false
	}
	return note, true
}

func (service *Service) tellRequesterHowTheUpdateEnded(ctx context.Context, note hostupdate.Note) error {
	personID, errorValue := service.hostUpdateRequesterPersonID(ctx, note.Requester)
	if errorValue != nil {
		return errorValue
	}
	body, errorValue := service.hostUpdateReportSchedule(ctx, note)
	if errorValue != nil {
		return errorValue
	}
	status, answer, errorValue := service.blueclawSignedAnswer(ctx, http.MethodPost, hostUpdateScheduleCreate, body, personID)
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

func (service *Service) hostUpdateRequesterPersonID(ctx context.Context, requester hostupdate.Requester) (string, error) {
	if personID := strings.TrimSpace(requester.PersonID); personID != "" {
		return personID, nil
	}
	personID, errorValue := service.resolveMemoryPersonIDFromPolicy(ctx, requester.Email)
	if errorValue == nil && personID == "" {
		return "", fmt.Errorf("the agent's policy does not name %s yet", requester.Email)
	}
	return personID, errorValue
}

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

func (service *Service) hostUpdateReportSchedule(ctx context.Context, note hostupdate.Note) ([]byte, error) {
	facts, errorValue := json.Marshal(hostUpdateResult(note))
	if errorValue != nil {
		return nil, errorValue
	}
	return json.Marshal(hostUpdateReportSchedule{
		TaskInstruction: "Tell the requester how the update of this company's host that they approved has ended, in one short message in this conversation. Use only these facts and invent none: " + string(facts),
		Description:     hostUpdateReportDescription,
		Kind:            "once",
		RunAt:           service.hostUpdate().Now().Add(hostUpdateReportDelay).UTC().Format(time.RFC3339),
		TimeZone:        service.companyTimeZoneName(ctx),
		Platform:        strings.TrimSpace(note.Requester.Platform),
		ConversationID:  strings.TrimSpace(note.Requester.ConversationID),
		ReplyTargetID:   strings.TrimSpace(note.Requester.ReplyTargetID),
	})
}

func hostUpdateResult(note hostupdate.Note) hostUpdateResultFacts {
	return hostUpdateResultFacts{
		FromVersion:      note.FromVersion,
		ToVersion:        note.ToVersion,
		StartedAt:        note.StartedAt.Format(time.RFC3339),
		FinishedAt:       note.Outcome.FinishedAt.Format(time.RFC3339),
		Succeeded:        note.Outcome.Succeeded,
		InstalledVersion: note.Outcome.InstalledVersion,
		Error:            note.Outcome.Error,
		InstallerOutput:  failedInstallerOutput(note.Outcome),
	}
}

func failedInstallerOutput(outcome *hostupdate.Outcome) string {
	if outcome.Succeeded {
		return ""
	}
	return outcome.OutputTail
}
