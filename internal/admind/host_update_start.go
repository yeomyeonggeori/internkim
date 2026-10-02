package admind

import (
	"net/http"
	"strings"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/hostupdate"
)

type hostUpdateStarted struct {
	Status                  string    `json:"status"`
	FromVersion             string    `json:"fromVersion"`
	ToVersion               string    `json:"toVersion"`
	StartedAt               time.Time `json:"startedAt"`
	ExpectedDowntimeMinutes int       `json:"expectedDowntimeMinutes"`
}

func (service *Service) startHostUpdate(responseWriter http.ResponseWriter, request *http.Request) {
	requesterEmail, isMember := service.hostRequesterEmail(responseWriter, request)
	if !isMember {
		return
	}
	var startRequest hostUpdateStartRequest
	if !decodeHostUpdateBody(responseWriter, request, &startRequest) {
		return
	}
	if !startRequest.IsApproved {
		(&hostUpdateRefusal{Status: http.StatusForbidden, ErrorCode: "approval_required", Message: "host_update requires the requester's approval before it runs"}).write(responseWriter)
		return
	}
	plan, refusal, errorValue := service.planHostUpdate(request.Context(), requesterEmail, startRequest.Input)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	if refusal != nil {
		refusal.write(responseWriter)
		return
	}
	started, errorValue := service.launchHostUpdate(plan, requesterOf(startRequest.Requester, requesterEmail))
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, started)
}

func requesterOf(declared hostupdate.Requester, requesterEmail string) hostupdate.Requester {
	declared.Email = strings.ToLower(strings.TrimSpace(requesterEmail))
	return declared
}

func (service *Service) launchHostUpdate(plan hostUpdatePlan, requester hostupdate.Requester) (hostUpdateStarted, error) {
	dependencies := service.hostUpdate()
	note := hostupdate.Note{Requester: requester, FromVersion: plan.FromVersion, ToVersion: plan.Target.Version, StartedAt: dependencies.Now().UTC()}
	if errorValue := hostupdate.WriteNote(dependencies.NotePath, note); errorValue != nil {
		return hostUpdateStarted{}, errorValue
	}
	if errorValue := dependencies.Supervisor.Start(dependencies.HostCommand, note.ToVersion); errorValue != nil {
		hostupdate.ClearNote(dependencies.NotePath)
		return hostUpdateStarted{}, errorValue
	}
	return hostUpdateStarted{
		Status:                  "started",
		FromVersion:             note.FromVersion,
		ToVersion:               note.ToVersion,
		StartedAt:               note.StartedAt,
		ExpectedDowntimeMinutes: expectedHostUpdateDowntimeMinutes,
	}, nil
}
