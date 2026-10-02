package admind

import (
	"context"
	"net/http"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/hostupdate"
)

type hostReleaseFact struct {
	Version     string    `json:"version"`
	PublishedAt time.Time `json:"publishedAt"`
	Notes       string    `json:"notes,omitempty"`
}

type hostUpdateInProgressFact struct {
	FromVersion string    `json:"fromVersion"`
	ToVersion   string    `json:"toVersion"`
	StartedAt   time.Time `json:"startedAt"`
}

type hostVersionAnswer struct {
	InstalledVersion  string                    `json:"installedVersion"`
	Channel           string                    `json:"channel"`
	UpdateMethod      string                    `json:"updateMethod"`
	LatestStable      *hostReleaseFact          `json:"latestStable,omitempty"`
	PreviousStable    *hostReleaseFact          `json:"previousStable,omitempty"`
	IsUpdateAvailable bool                      `json:"isUpdateAvailable"`
	UpdateInProgress  *hostUpdateInProgressFact `json:"updateInProgress,omitempty"`
}

func (service *Service) answerHostVersion(responseWriter http.ResponseWriter, request *http.Request) {
	if _, isMember := service.hostRequesterEmail(responseWriter, request); !isMember {
		return
	}
	answer, errorValue := service.hostVersion(request.Context())
	if errorValue != nil {
		http.Error(responseWriter, errorValue.Error(), http.StatusBadGateway)
		return
	}
	service.writeJSON(responseWriter, answer)
}

func (service *Service) hostVersion(ctx context.Context) (hostVersionAnswer, error) {
	dependencies := service.hostUpdate()
	answer := hostVersionAnswer{
		InstalledVersion: dependencies.Machine.InstalledVersion(),
		Channel:          dependencies.Machine.Channel(),
		UpdateMethod:     dependencies.Machine.UpdateMethod(),
		UpdateInProgress: unfinishedUpdate(dependencies.NotePath),
	}
	latest, hasLatest, errorValue := dependencies.Releases.LatestStable(ctx)
	if errorValue != nil {
		return hostVersionAnswer{}, errorValue
	}
	if hasLatest {
		answer.LatestStable = releaseFact(latest, true)
		answer.IsUpdateAvailable = hostupdate.IsOlder(answer.InstalledVersion, latest.Version)
	}
	stable, errorValue := dependencies.Releases.StableReleases(ctx)
	if errorValue != nil {
		return hostVersionAnswer{}, errorValue
	}
	if previous, hasPrevious := hostupdate.NewestOlderThan(stable, answer.InstalledVersion); hasPrevious {
		answer.PreviousStable = releaseFact(previous, false)
	}
	return answer, nil
}

func releaseFact(release hostupdate.Release, carriesNotes bool) *hostReleaseFact {
	fact := &hostReleaseFact{Version: release.Version, PublishedAt: release.PublishedAt}
	if carriesNotes {
		fact.Notes = clippedReleaseNotes(release.Notes)
	}
	return fact
}

func unfinishedUpdate(notePath string) *hostUpdateInProgressFact {
	note, isPending, errorValue := hostupdate.ReadNote(notePath)
	if errorValue != nil || !isPending || note.IsFinished() {
		return nil
	}
	return &hostUpdateInProgressFact{FromVersion: note.FromVersion, ToVersion: note.ToVersion, StartedAt: note.StartedAt}
}
