package admind

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/capabilities"
	"github.com/yeomyeonggeori/internkim/internal/hostupdate"
)

const (
	hostUpdateChoiceNow           = "now"
	hostUpdateChoiceOffHours      = "offHours"
	hostUpdateChoiceRequestedTime = "requestedTime"
)

type hostUpdateChoice struct {
	Key      string `json:"key"`
	StartsAt string `json:"startsAt,omitempty"`
}

type hostUpdatePlan struct {
	FromVersion string
	Target      hostupdate.Release
	Choices     []hostUpdateChoice
}

type hostUpdateConsequences struct {
	FromVersion                  string   `json:"fromVersion"`
	ToVersion                    string   `json:"toVersion"`
	IsRollback                   bool     `json:"isRollback"`
	ReleasePublishedAt           string   `json:"releasePublishedAt"`
	ReleaseNotes                 string   `json:"releaseNotes,omitempty"`
	ServicesThatRestart          []string `json:"servicesThatRestart"`
	ExpectedDowntimeMinutes      int      `json:"expectedDowntimeMinutes"`
	MessagesSentDuringTheUpdate  string   `json:"messagesSentDuringTheUpdate"`
	TasksRunningAtTheRestart     string   `json:"tasksRunningAtTheRestart"`
	ScheduledJobsDueDuringUpdate string   `json:"scheduledJobsDueDuringTheUpdate"`
	IfDownloadOrCheckFails       string   `json:"ifTheDownloadOrChecksumFails"`
	IfTheNewVersionFailsToStart  string   `json:"ifTheNewVersionFailsToStart"`
	OutcomeReport                string   `json:"outcomeReport"`
}

type hostUpdateTarget struct {
	capabilities.ApprovalTarget
	Choices []hostUpdateChoice `json:"choices"`
}

func (service *Service) answerHostUpdatePlan(responseWriter http.ResponseWriter, request *http.Request) {
	requesterEmail, isMember := service.hostRequesterEmail(responseWriter, request)
	if !isMember {
		return
	}
	var input hostUpdateInput
	if !decodeHostUpdateBody(responseWriter, request, &input) {
		return
	}
	plan, refusal, errorValue := service.planHostUpdate(request.Context(), requesterEmail, input)
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	if refusal == nil {
		plan.Choices, refusal = service.hostUpdateChoices(request.Context(), service.hostUpdate().Now(), input)
	}
	if refusal != nil {
		refusal.write(responseWriter)
		return
	}
	service.writeJSON(responseWriter, plan.approvalTarget())
}

func (service *Service) planHostUpdate(ctx context.Context, requesterEmail string, input hostUpdateInput) (hostUpdatePlan, *hostUpdateRefusal, error) {
	if !service.isCurrentAdminEmail(ctx, requesterEmail) {
		return hostUpdatePlan{}, refusedNotAdministrator(), nil
	}
	dependencies := service.hostUpdate()
	if refusal := refusalForThisHost(dependencies); refusal != nil {
		return hostUpdatePlan{}, refusal, nil
	}
	fromVersion := dependencies.Machine.InstalledVersion()
	target, refusal, errorValue := chooseTargetRelease(ctx, dependencies.Releases, fromVersion, input.TargetVersion)
	if errorValue != nil || refusal != nil {
		return hostUpdatePlan{}, refusal, errorValue
	}
	return hostUpdatePlan{FromVersion: fromVersion, Target: target}, nil, nil
}

func refusedNotAdministrator() *hostUpdateRefusal {
	return &hostUpdateRefusal{Status: http.StatusForbidden, ErrorCode: "access_denied", Message: "only an administrator of this company can update its host"}
}

func refusalForThisHost(dependencies hostUpdateDependencies) *hostUpdateRefusal {
	method := dependencies.Machine.UpdateMethod()
	switch {
	case method == hostupdate.MethodBrew:
		return &hostUpdateRefusal{Status: http.StatusConflict, ErrorCode: "update_method_unsupported",
			Message: "the agent does not update a Mac host; an administrator runs brew upgrade internkim and then sudo internkim refresh on it",
			Facts:   map[string]string{"updateMethod": method}}
	case method == "" || dependencies.Machine.InstalledVersion() == "":
		return &hostUpdateRefusal{Status: http.StatusConflict, ErrorCode: "not_a_packaged_host", Message: "this machine runs no internkim package, so there is nothing to update"}
	}
	if channel := dependencies.Machine.Channel(); channel != hostupdate.ChannelStable {
		return &hostUpdateRefusal{Status: http.StatusConflict, ErrorCode: "channel_not_stable",
			Message: "the agent updates only a host that follows the stable channel, and this one follows " + channel,
			Facts:   map[string]string{"channel": channel}}
	}
	if isHostUpdateUnderway(dependencies) {
		return &hostUpdateRefusal{Status: http.StatusConflict, ErrorCode: "update_in_progress", Message: "an update of this host is already running"}
	}
	return nil
}

func isHostUpdateUnderway(dependencies hostUpdateDependencies) bool {
	note, isPending, errorValue := hostupdate.ReadNote(dependencies.NotePath)
	if errorValue == nil && isPending && !note.IsFinished() {
		return true
	}
	return dependencies.Supervisor.IsRunning()
}

func chooseTargetRelease(ctx context.Context, releases hostupdate.ReleaseSource, fromVersion string, requested string) (hostupdate.Release, *hostUpdateRefusal, error) {
	target, refusal, errorValue := requestedOrLatestRelease(ctx, releases, strings.TrimSpace(requested))
	if errorValue != nil || refusal != nil {
		return hostupdate.Release{}, refusal, errorValue
	}
	if target.Version == fromVersion {
		return hostupdate.Release{}, &hostUpdateRefusal{Status: http.StatusConflict, ErrorCode: "already_installed",
			Message: "this host already runs " + fromVersion, Facts: map[string]string{"installedVersion": fromVersion}}, nil
	}
	return target, nil, nil
}

func requestedOrLatestRelease(ctx context.Context, releases hostupdate.ReleaseSource, requested string) (hostupdate.Release, *hostUpdateRefusal, error) {
	if requested == "" {
		latest, hasLatest, errorValue := releases.LatestStable(ctx)
		if errorValue != nil || hasLatest {
			return latest, nil, errorValue
		}
		return hostupdate.Release{}, &hostUpdateRefusal{Status: http.StatusNotFound, ErrorCode: "no_stable_release", Message: "no stable release has been published yet"}, nil
	}
	stable, errorValue := releases.StableReleases(ctx)
	if errorValue != nil {
		return hostupdate.Release{}, nil, errorValue
	}
	if release, isStable := hostupdate.FindRelease(stable, hostupdate.TagOf(requested)); isStable {
		return release, nil, nil
	}
	return hostupdate.Release{}, &hostUpdateRefusal{Status: http.StatusNotFound, ErrorCode: "unknown_stable_release",
		Message: requested + " is not a stable release", Facts: map[string]string{"stableVersions": stableVersionList(stable)}}, nil
}

func stableVersionList(releases []hostupdate.Release) string {
	versions := []string{}
	for _, release := range releases {
		versions = append(versions, release.Version)
	}
	return strings.Join(versions, ", ")
}

func (service *Service) hostUpdateChoices(ctx context.Context, now time.Time, input hostUpdateInput) ([]hostUpdateChoice, *hostUpdateRefusal) {
	immediately := hostUpdateChoice{Key: hostUpdateChoiceNow}
	if requested := strings.TrimSpace(input.StartsAt); requested != "" {
		startsAt, errorValue := time.Parse(time.RFC3339, requested)
		if errorValue != nil || !startsAt.After(now) {
			return nil, &hostUpdateRefusal{Status: http.StatusBadRequest, ErrorCode: "invalid_start_time",
				Message: "startsAt must be a future instant in RFC 3339, and was " + requested}
		}
		return []hostUpdateChoice{{Key: hostUpdateChoiceRequestedTime, StartsAt: startsAt.Format(time.RFC3339)}, immediately}, nil
	}
	offHours := hostUpdateChoice{Key: hostUpdateChoiceOffHours, StartsAt: nextOffHoursStart(now, service.companyTimeLocation(ctx)).Format(time.RFC3339)}
	if input.IsRequestedNow {
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

func (plan hostUpdatePlan) approvalTarget() hostUpdateTarget {
	preview, _ := json.Marshal(plan.consequences())
	return hostUpdateTarget{
		ApprovalTarget: capabilities.ApprovalTarget{
			InputField: "targetVersion",
			ID:         plan.Target.Version,
			Title:      plan.FromVersion + " → " + plan.Target.Version,
			Preview:    string(preview),
		},
		Choices: plan.Choices,
	}
}

func (plan hostUpdatePlan) consequences() hostUpdateConsequences {
	return hostUpdateConsequences{
		FromVersion:                  plan.FromVersion,
		ToVersion:                    plan.Target.Version,
		IsRollback:                   hostupdate.IsOlder(plan.Target.Version, plan.FromVersion),
		ReleasePublishedAt:           plan.Target.PublishedAt.Format(time.RFC3339),
		ReleaseNotes:                 clippedReleaseNotes(plan.Target.Notes),
		ServicesThatRestart:          []string{"agent", "relay", "messenger"},
		ExpectedDowntimeMinutes:      expectedHostUpdateDowntimeMinutes,
		MessagesSentDuringTheUpdate:  "they wait in the relay's queue and are answered after the restart",
		TasksRunningAtTheRestart:     "each is interrupted and resumed once automatically when the agent is back; one that cannot be resumed is marked failed and its requester is told",
		ScheduledJobsDueDuringUpdate: "each runs once, late, when the agent is back",
		IfDownloadOrCheckFails:       "nothing is installed and the host stays on " + plan.FromVersion,
		IfTheNewVersionFailsToStart:  "the package manager leaves " + plan.Target.Version + " installed but unconfigured and the host does not go back by itself; an administrator reinstalls " + plan.FromVersion + " with the one-line installer and --version " + plan.FromVersion,
		OutcomeReport:                "the agent replies in this conversation with the result once it is back",
	}
}
