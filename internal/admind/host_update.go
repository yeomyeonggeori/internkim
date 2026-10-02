package admind

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/hostupdate"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

const (
	hostVersionPath      = "/host/api/version"
	hostUpdatePlanPath   = "/host/api/update/plan"
	hostUpdateStartPath  = "/host/api/update"
	hostUpdateBodyLimit  = 16 * 1024
	hostUpdateNotesLimit = 4000

	expectedHostUpdateDowntimeMinutes = 3
	offHoursStartHour                 = 3
)

type hostUpdateDependencies struct {
	Machine       hostupdate.Machine
	Releases      hostupdate.ReleaseSource
	Supervisor    hostupdate.Supervisor
	NotePath      string
	HostCommand   string
	Now           func() time.Time
	IsInitialized bool
}

func (service *Service) hostUpdate() hostUpdateDependencies {
	if service.hostUpdateDependencies.IsInitialized {
		return service.hostUpdateDependencies
	}
	return hostUpdateDependencies{
		Machine:     hostupdate.LocalMachine(BuildID),
		Releases:    hostupdate.DefaultReleaseSource(),
		Supervisor:  hostupdate.LocalSupervisor(),
		NotePath:    hostupdate.NotePath,
		HostCommand: hostupdate.HostCommandPath(blueclaw.CompanyPackageBinaryRoot),
		Now:         time.Now,
	}
}

type hostUpdateInput struct {
	TargetVersion  string `json:"targetVersion,omitempty"`
	StartsAt       string `json:"startsAt,omitempty"`
	IsRequestedNow bool   `json:"isRequestedNow,omitempty"`
}

type hostUpdateStartRequest struct {
	Input      hostUpdateInput      `json:"input"`
	Requester  hostupdate.Requester `json:"requester"`
	IsApproved bool                 `json:"isApproved"`
}

type hostUpdateRefusal struct {
	Status    int
	ErrorCode string
	Message   string
	Facts     map[string]string
}

func (refusal hostUpdateRefusal) write(responseWriter http.ResponseWriter) {
	document := map[string]any{"errorCode": refusal.ErrorCode, "failureStage": "authorization", "message": refusal.Message, "retryable": false}
	if refusal.Status != http.StatusForbidden {
		document["failureStage"] = "precondition"
	}
	for name, value := range refusal.Facts {
		document[name] = value
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(refusal.Status)
	json.NewEncoder(responseWriter).Encode(document)
}

func (service *Service) registerHostRoutes(multiplexer *http.ServeMux) {
	multiplexer.HandleFunc(hostVersionPath, service.answerHostVersion)
	multiplexer.HandleFunc(hostUpdatePlanPath, service.answerHostUpdatePlan)
	multiplexer.HandleFunc(hostUpdateStartPath, service.startHostUpdate)
}

func (service *Service) hostRequesterEmail(responseWriter http.ResponseWriter, request *http.Request) (string, bool) {
	request.Header.Del(taskResolvedActorHeader)
	if request.Method != http.MethodPost || !arrivedOnRequesterSocket(request) || !service.authorizeTaskRequest(request) {
		http.Error(responseWriter, "company membership required", http.StatusForbidden)
		return "", false
	}
	return service.taskActorEmail(request), true
}

func decodeHostUpdateBody(responseWriter http.ResponseWriter, request *http.Request, into any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(responseWriter, request.Body, hostUpdateBodyLimit))
	decoder.DisallowUnknownFields()
	if errorValue := decoder.Decode(into); errorValue != nil && !errors.Is(errorValue, io.EOF) {
		http.Error(responseWriter, "that is not a host update request this host can read: "+errorValue.Error(), http.StatusBadRequest)
		return false
	}
	return true
}

func clippedReleaseNotes(notes string) string {
	runes := []rune(strings.TrimSpace(notes))
	if len(runes) <= hostUpdateNotesLimit {
		return string(runes)
	}
	return string(runes[:hostUpdateNotesLimit]) + "…"
}
