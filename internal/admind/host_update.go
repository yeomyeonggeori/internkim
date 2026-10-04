package admind

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
	"github.com/yeomyeonggeori/internkim/internal/hostupdate"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

const (
	hostVersionPath     = "/host/api/version"
	hostUpdatePlanPath  = "/host/api/update/plan"
	hostUpdateStartPath = "/host/api/update"

	expectedHostUpdateDowntimeSeconds = 60
	offHoursStartHour                 = 3
	releaseNotesLimit                 = 4000
	stableReleaseFreshness            = 5 * time.Minute
)

type hostUpdateDependencies struct {
	Machine     hostupdate.Machine
	Releases    hostupdate.ReleaseSource
	Supervisor  hostupdate.Supervisor
	NotePath    string
	HostCommand string
	Now         func() time.Time
}

func (service *Service) hostUpdate() hostUpdateDependencies {
	if service.hostUpdateDependencies.Now != nil {
		return service.hostUpdateDependencies
	}
	return hostUpdateDependencies{
		Machine:     hostupdate.LocalMachine(BuildID),
		Releases:    hostupdate.DefaultReleaseSource(),
		Supervisor:  hostupdate.LocalSupervisor(),
		NotePath:    hostupdate.NotePath,
		HostCommand: blueclaw.CompanyPackageBinaryRoot + "/internkim",
		Now:         time.Now,
	}
}

type heldStableReleases struct {
	mutex     sync.Mutex
	releases  []hostupdate.Release
	fetchedAt time.Time
	isHeld    bool
}

func (service *Service) stableReleases(ctx context.Context) ([]hostupdate.Release, error) {
	dependencies := service.hostUpdate()
	held := &service.stableReleaseCache
	held.mutex.Lock()
	defer held.mutex.Unlock()
	if held.isHeld && dependencies.Now().Sub(held.fetchedAt) < stableReleaseFreshness {
		return held.releases, nil
	}
	releases, errorValue := dependencies.Releases.StableReleases(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	held.releases, held.fetchedAt, held.isHeld = releases, dependencies.Now(), true
	return releases, nil
}

type hostRequest struct {
	Input struct {
		TargetVersion  string `json:"targetVersion,omitempty"`
		StartsAt       string `json:"startsAt,omitempty"`
		IsRequestedNow bool   `json:"isRequestedNow,omitempty"`
	} `json:"input"`
	Requester  hostupdate.Requester `json:"requester"`
	IsApproved bool                 `json:"isApproved"`
}

type hostRefusal struct {
	Status    int
	ErrorCode string
	Message   string
}

type hostUpdateChoice struct {
	Key      string `json:"key"`
	StartsAt string `json:"startsAt,omitempty"`
}

type hostUpdatePlan struct {
	FromVersion string
	Target      hostupdate.Release
}

type hostAnswer func(ctx context.Context, requesterEmail string, request hostRequest) (any, *hostRefusal, error)

func (service *Service) registerHostRoutes(multiplexer *http.ServeMux) {
	multiplexer.HandleFunc(hostVersionPath, service.hostRoute(service.hostVersion))
	multiplexer.HandleFunc(hostUpdatePlanPath, service.hostRoute(service.hostUpdateConfirmation))
	multiplexer.HandleFunc(hostUpdateStartPath, service.hostRoute(service.startHostUpdate))
}

func (service *Service) hostRoute(answer hostAnswer) http.HandlerFunc {
	return func(responseWriter http.ResponseWriter, request *http.Request) {
		request.Header.Del(taskResolvedActorHeader)
		if request.Method != http.MethodPost || !arrivedOnRequesterSocket(request) || !service.authorizeTaskRequest(request) {
			http.Error(responseWriter, "company membership required", http.StatusForbidden)
			return
		}
		var body hostRequest
		decoder := json.NewDecoder(http.MaxBytesReader(responseWriter, request.Body, 16*1024))
		decoder.DisallowUnknownFields()
		if errorValue := decoder.Decode(&body); errorValue != nil && !errors.Is(errorValue, io.EOF) {
			http.Error(responseWriter, "that is not a host request this host can read: "+errorValue.Error(), http.StatusBadRequest)
			return
		}
		answered, refusal, errorValue := answer(request.Context(), service.taskActorEmail(request), body)
		switch {
		case errorValue != nil:
			http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		case refusal != nil:
			refusal.write(responseWriter)
		default:
			service.writeJSON(responseWriter, answered)
		}
	}
}

func (refusal hostRefusal) write(responseWriter http.ResponseWriter) {
	stage := "precondition"
	if refusal.Status == http.StatusForbidden {
		stage = "authorization"
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(refusal.Status)
	json.NewEncoder(responseWriter).Encode(map[string]any{"errorCode": refusal.ErrorCode, "failureStage": stage, "message": refusal.Message, "retryable": false})
}

func (service *Service) hostVersion(ctx context.Context, _ string, _ hostRequest) (any, *hostRefusal, error) {
	dependencies := service.hostUpdate()
	stable, errorValue := service.stableReleases(ctx)
	if errorValue != nil {
		return nil, nil, errorValue
	}
	installed := dependencies.Machine.InstalledVersion
	answer := map[string]any{
		"installedVersion":        installed,
		"channel":                 dependencies.Machine.Channel(),
		"updateMethod":            dependencies.Machine.UpdateMethod(),
		"isUpdateAvailable":       len(stable) > 0 && hostupdate.IsOlder(installed, stable[0].Version),
		"expectedDowntimeSeconds": expectedHostUpdateDowntimeSeconds,
	}
	if len(stable) > 0 {
		answer["latestStable"] = withClippedNotes(stable[0])
	}
	for _, release := range stable {
		if hostupdate.IsOlder(release.Version, installed) {
			answer["previousStable"] = hostupdate.Release{Version: release.Version, PublishedAt: release.PublishedAt}
			break
		}
	}
	if note, isPending, _ := hostupdate.ReadNote(dependencies.NotePath); isPending {
		describeTheNote(answer, note)
	}
	return answer, nil, nil
}

func describeTheNote(answer map[string]any, note hostupdate.Note) {
	if !note.IsFinished() {
		answer["updateInProgress"] = map[string]any{"fromVersion": note.FromVersion, "toVersion": note.ToVersion, "startedAt": note.StartedAt}
		return
	}
	lastUpdate := map[string]any{
		"fromVersion": note.FromVersion,
		"toVersion":   note.ToVersion,
		"startedAt":   note.StartedAt,
		"finishedAt":  note.Outcome.FinishedAt,
		"succeeded":   note.Outcome.Succeeded,
	}
	if note.Outcome.Error != "" {
		lastUpdate["error"] = note.Outcome.Error
	}
	answer["lastUpdate"] = lastUpdate
}

func withClippedNotes(release hostupdate.Release) hostupdate.Release {
	if runes := []rune(release.Notes); len(runes) > releaseNotesLimit {
		release.Notes = string(runes[:releaseNotesLimit]) + "…"
	}
	return release
}

func (service *Service) hostUpdateConfirmation(ctx context.Context, requesterEmail string, request hostRequest) (any, *hostRefusal, error) {
	plan, refusal, errorValue := service.planHostUpdate(ctx, requesterEmail, request)
	if errorValue != nil || refusal != nil {
		return nil, refusal, errorValue
	}
	choices, refusal := service.hostUpdateChoices(ctx, request)
	if refusal != nil {
		return nil, refusal, nil
	}
	preview, _ := json.Marshal(plan.consequences())
	return struct {
		capabilities.ApprovalTarget
		Choices []hostUpdateChoice `json:"choices"`
	}{
		ApprovalTarget: capabilities.ApprovalTarget{InputField: "targetVersion", ID: plan.Target.Version, Title: plan.FromVersion + " → " + plan.Target.Version, Preview: string(preview)},
		Choices:        choices,
	}, nil, nil
}

func (service *Service) planHostUpdate(ctx context.Context, requesterEmail string, request hostRequest) (hostUpdatePlan, *hostRefusal, error) {
	if !service.isCurrentAdminEmail(ctx, requesterEmail) {
		return hostUpdatePlan{}, &hostRefusal{http.StatusForbidden, "access_denied", "only an administrator of this company can update its host"}, nil
	}
	dependencies := service.hostUpdate()
	if refusal := refusalForThisHost(dependencies); refusal != nil {
		return hostUpdatePlan{}, refusal, nil
	}
	stable, errorValue := service.stableReleases(ctx)
	if errorValue != nil {
		return hostUpdatePlan{}, nil, errorValue
	}
	target, refusal := chooseTarget(stable, strings.TrimSpace(request.Input.TargetVersion), dependencies.Machine.InstalledVersion)
	return hostUpdatePlan{FromVersion: dependencies.Machine.InstalledVersion, Target: target}, refusal, nil
}

func refusalForThisHost(dependencies hostUpdateDependencies) *hostRefusal {
	method := dependencies.Machine.UpdateMethod()
	channel := dependencies.Machine.Channel()
	switch {
	case method == hostupdate.MethodBrew:
		return &hostRefusal{http.StatusConflict, "update_method_unsupported", "the agent does not update a Mac host; an administrator runs brew upgrade internkim on it, which asks for their password to restart the services"}
	case method == "":
		return &hostRefusal{http.StatusConflict, "not_a_packaged_host", "this machine runs no internkim package, so there is nothing to update"}
	case channel != hostupdate.ChannelStable:
		return &hostRefusal{http.StatusConflict, "channel_not_stable", "the agent updates only a host that follows the stable channel, and this one follows " + channel}
	}
	if note, isPending, _ := hostupdate.ReadNote(dependencies.NotePath); (isPending && !note.IsFinished()) || dependencies.Supervisor.IsRunning() {
		return &hostRefusal{http.StatusConflict, "update_in_progress", "an update of this host is already running"}
	}
	return nil
}

func chooseTarget(stable []hostupdate.Release, requested string, installed string) (hostupdate.Release, *hostRefusal) {
	if len(stable) == 0 {
		return hostupdate.Release{}, &hostRefusal{http.StatusNotFound, "no_stable_release", "no stable release has been published yet"}
	}
	target := stable[0]
	if requested != "" {
		target = hostupdate.Release{}
		versions := []string{}
		for _, release := range stable {
			versions = append(versions, release.Version)
			if release.Version == hostupdate.TagOf(requested) {
				target = release
			}
		}
		if target.Version == "" {
			return target, &hostRefusal{http.StatusNotFound, "unknown_stable_release", requested + " is not a stable release; the stable releases are " + strings.Join(versions, ", ")}
		}
	}
	if target.Version == installed {
		return target, &hostRefusal{http.StatusConflict, "already_installed", "this host already runs " + installed}
	}
	return target, nil
}

func (service *Service) hostUpdateChoices(ctx context.Context, request hostRequest) ([]hostUpdateChoice, *hostRefusal) {
	now := service.hostUpdate().Now()
	immediately := hostUpdateChoice{Key: "now"}
	if requested := strings.TrimSpace(request.Input.StartsAt); requested != "" {
		startsAt, errorValue := time.Parse(time.RFC3339, requested)
		if errorValue != nil || !startsAt.After(now) {
			return nil, &hostRefusal{http.StatusBadRequest, "invalid_start_time", "startsAt must be a future instant in RFC 3339, and was " + requested}
		}
		return []hostUpdateChoice{{Key: "requestedTime", StartsAt: startsAt.Format(time.RFC3339)}, immediately}, nil
	}
	offHours := hostUpdateChoice{Key: "offHours", StartsAt: nextOffHoursStart(now, service.companyTimeLocation(ctx)).Format(time.RFC3339)}
	if request.Input.IsRequestedNow {
		return []hostUpdateChoice{immediately, offHours}, nil
	}
	return []hostUpdateChoice{offHours, immediately}, nil
}

func nextOffHoursStart(now time.Time, location *time.Location) time.Time {
	local := now.In(location)
	start := time.Date(local.Year(), local.Month(), local.Day(), offHoursStartHour, 0, 0, 0, location)
	if !start.After(local) {
		start = start.AddDate(0, 0, 1)
	}
	return start
}

func (plan hostUpdatePlan) consequences() map[string]any {
	return map[string]any{
		"fromVersion":                     plan.FromVersion,
		"toVersion":                       plan.Target.Version,
		"isRollback":                      hostupdate.IsOlder(plan.Target.Version, plan.FromVersion),
		"releasePublishedAt":              plan.Target.PublishedAt,
		"releaseNotes":                    withClippedNotes(plan.Target).Notes,
		"servicesThatRestart":             []string{"agent", "relay", "messenger"},
		"expectedDowntimeSeconds":         expectedHostUpdateDowntimeSeconds,
		"messagesSentDuringTheUpdate":     "while the messenger bridge restarts, sending a message fails and it has to be sent again",
		"tasksRunningAtTheRestart":        "each is interrupted and resumed once automatically when the agent is back; one that cannot be resumed is marked failed and its requester is told",
		"scheduledJobsDueDuringTheUpdate": "each runs once, late, when the agent is back",
		"ifTheDownloadOrChecksumFails":    "nothing is installed and the host stays on " + plan.FromVersion,
		"ifTheNewVersionFailsToStart":     "the package manager leaves " + plan.Target.Version + " installed but unconfigured and the host does not go back by itself; an administrator reinstalls " + plan.FromVersion + " with the one-line installer and --version " + plan.FromVersion,
		"outcomeReport":                   "the agent replies in this conversation with the result once it is back",
	}
}

func (service *Service) startHostUpdate(ctx context.Context, requesterEmail string, request hostRequest) (any, *hostRefusal, error) {
	if !request.IsApproved {
		return nil, &hostRefusal{http.StatusForbidden, "approval_required", "host_update requires the requester's approval before it runs"}, nil
	}
	plan, refusal, errorValue := service.planHostUpdate(ctx, requesterEmail, request)
	if errorValue != nil || refusal != nil {
		return nil, refusal, errorValue
	}
	dependencies := service.hostUpdate()
	requester := request.Requester
	requester.Email = requesterEmail
	note := hostupdate.Note{Requester: requester, FromVersion: plan.FromVersion, ToVersion: plan.Target.Version, StartedAt: dependencies.Now().UTC()}
	if errorValue := hostupdate.WriteNote(dependencies.NotePath, note); errorValue != nil {
		return nil, nil, errorValue
	}
	if errorValue := dependencies.Supervisor.Start(dependencies.HostCommand, note.ToVersion); errorValue != nil {
		hostupdate.ClearNote(dependencies.NotePath)
		return nil, nil, errorValue
	}
	return map[string]any{
		"status":                  "started",
		"fromVersion":             note.FromVersion,
		"toVersion":               note.ToVersion,
		"startedAt":               note.StartedAt,
		"expectedDowntimeSeconds": expectedHostUpdateDowntimeSeconds,
	}, nil, nil
}
