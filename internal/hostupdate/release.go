package hostupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	Repository                 = "yeomyeonggeori/internkim"
	ReleaseAPIOverrideVariable = "INTERNKIM_RELEASE_API_URL"
	releaseListPageSize        = 30
	releaseResponseCeiling     = 4 << 20
)

type Release struct {
	Version     string    `json:"version"`
	PublishedAt time.Time `json:"publishedAt"`
	Notes       string    `json:"notes,omitempty"`
}

type ReleaseSource struct {
	APIURL     string
	HTTPClient *http.Client
}

type githubRelease struct {
	TagName      string    `json:"tag_name"`
	PublishedAt  time.Time `json:"published_at"`
	Body         string    `json:"body"`
	IsPrerelease bool      `json:"prerelease"`
	IsDraft      bool      `json:"draft"`
}

func DefaultReleaseSource() ReleaseSource {
	apiURL := strings.TrimSpace(os.Getenv(ReleaseAPIOverrideVariable))
	if apiURL == "" {
		apiURL = "https://api.github.com/repos/" + Repository
	}
	return ReleaseSource{APIURL: strings.TrimRight(apiURL, "/"), HTTPClient: &http.Client{Timeout: 20 * time.Second}}
}

func (source ReleaseSource) LatestStable(ctx context.Context) (Release, bool, error) {
	var latest githubRelease
	isFound, errorValue := source.get(ctx, "/releases/latest", &latest)
	if errorValue != nil || !isFound || latest.IsPrerelease || latest.IsDraft {
		return Release{}, false, errorValue
	}
	return latest.release(), true, nil
}

func (source ReleaseSource) StableReleases(ctx context.Context) ([]Release, error) {
	var listed []githubRelease
	if _, errorValue := source.get(ctx, fmt.Sprintf("/releases?per_page=%d", releaseListPageSize), &listed); errorValue != nil {
		return nil, errorValue
	}
	stable := []Release{}
	for _, release := range listed {
		if !release.IsPrerelease && !release.IsDraft {
			stable = append(stable, release.release())
		}
	}
	return stable, nil
}

func (release githubRelease) release() Release {
	return Release{Version: strings.TrimSpace(release.TagName), PublishedAt: release.PublishedAt, Notes: strings.TrimSpace(release.Body)}
}

func (source ReleaseSource) get(ctx context.Context, path string, answer any) (bool, error) {
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, source.APIURL+path, nil)
	if errorValue != nil {
		return false, errorValue
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	response, errorValue := source.client().Do(request)
	if errorValue != nil {
		return false, fmt.Errorf("could not reach the release list at %s: %w", source.APIURL, errorValue)
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return false, nil
	}
	if response.StatusCode != http.StatusOK {
		return false, fmt.Errorf("the release list at %s answered %d", source.APIURL+path, response.StatusCode)
	}
	body, errorValue := io.ReadAll(io.LimitReader(response.Body, releaseResponseCeiling))
	if errorValue != nil {
		return false, errorValue
	}
	return true, json.Unmarshal(body, answer)
}

func (source ReleaseSource) client() *http.Client {
	if source.HTTPClient != nil {
		return source.HTTPClient
	}
	return http.DefaultClient
}

func FindRelease(releases []Release, version string) (Release, bool) {
	for _, release := range releases {
		if release.Version == version {
			return release, true
		}
	}
	return Release{}, false
}

func NewestOlderThan(releases []Release, version string) (Release, bool) {
	newest := Release{}
	for _, release := range releases {
		if IsOlder(release.Version, version) && IsOlder(newest.Version, release.Version) {
			newest = release
		}
	}
	return newest, newest.Version != ""
}

func IsOlder(version string, than string) bool {
	return strings.TrimPrefix(version, "v") < strings.TrimPrefix(than, "v")
}

func TagOf(packageVersion string) string {
	trimmed := strings.TrimSpace(packageVersion)
	if trimmed == "" || strings.HasPrefix(trimmed, "v") {
		return trimmed
	}
	return "v" + trimmed
}
