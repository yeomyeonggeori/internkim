package hostupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

const (
	Repository                 = "yeomyeonggeori/internkim"
	ReleaseAPIOverrideVariable = "INTERNKIM_RELEASE_API_URL"
)

type Release struct {
	Version     string    `json:"version"`
	PublishedAt time.Time `json:"publishedAt"`
	Notes       string    `json:"notes,omitempty"`
}

type ReleaseSource struct {
	APIURL string
}

func DefaultReleaseSource() ReleaseSource {
	if apiURL := strings.TrimSpace(os.Getenv(ReleaseAPIOverrideVariable)); apiURL != "" {
		return ReleaseSource{APIURL: strings.TrimRight(apiURL, "/")}
	}
	return ReleaseSource{APIURL: "https://api.github.com/repos/" + Repository}
}

func (source ReleaseSource) StableReleases(ctx context.Context) ([]Release, error) {
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet, source.APIURL+"/releases?per_page=30", nil)
	if errorValue != nil {
		return nil, errorValue
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	response, errorValue := (&http.Client{Timeout: 20 * time.Second}).Do(request)
	if errorValue != nil {
		return nil, fmt.Errorf("could not reach the release list at %s: %w", source.APIURL, errorValue)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("the release list at %s answered %d", source.APIURL, response.StatusCode)
	}
	var listed []struct {
		TagName      string    `json:"tag_name"`
		PublishedAt  time.Time `json:"published_at"`
		Body         string    `json:"body"`
		IsPrerelease bool      `json:"prerelease"`
		IsDraft      bool      `json:"draft"`
	}
	if errorValue := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(&listed); errorValue != nil {
		return nil, errorValue
	}
	stable := []Release{}
	for _, release := range listed {
		if !release.IsPrerelease && !release.IsDraft {
			stable = append(stable, Release{Version: release.TagName, PublishedAt: release.PublishedAt, Notes: strings.TrimSpace(release.Body)})
		}
	}
	sort.Slice(stable, func(left int, right int) bool { return IsOlder(stable[right].Version, stable[left].Version) })
	return stable, nil
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
