package centralplane

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// A clock as the company holds it, with the person as an address so a device
// can turn them into whatever identifier its own directory uses.
type Clock struct {
	Email      string
	Name       string
	Kind       string
	OccurredAt time.Time
}

const clockSelection = "kind,occurred_at,member:member_id(email,name)"

// ClocksSince answers every attendance record the company holds from a moment
// onward, read as the person who asked so row level security decides what comes
// back. occurred_at already carries a correction, so a record somebody fixed
// answers with the moment it was fixed to.
func (client *Client) ClocksSince(ctx context.Context, platform string, externalID string, from time.Time) ([]Clock, error) {
	session, errorValue := client.sessionFor(ctx, platform, externalID)
	if errorValue != nil {
		return nil, errorValue
	}

	query := url.Values{}
	query.Set("select", clockSelection)
	query.Set("order", "occurred_at.asc")
	query.Set("occurred_at", "gte."+from.UTC().Format(time.RFC3339))
	query.Set("deleted_at", "is.null")

	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet,
		strings.TrimSuffix(client.settings.ProjectURL, "/")+"/rest/v1/attendance?"+query.Encode(), nil)
	if errorValue != nil {
		return nil, errorValue
	}
	client.signAsMember(request, session)

	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return nil, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return nil, fmt.Errorf("central plane refused to say who clocked in: %s", response.Status)
	}

	var rows []struct {
		Kind       string `json:"kind"`
		OccurredAt string `json:"occurred_at"`
		Member     *struct {
			Email string `json:"email"`
			Name  string `json:"name"`
		} `json:"member"`
	}
	if errorValue := json.NewDecoder(response.Body).Decode(&rows); errorValue != nil {
		return nil, errorValue
	}

	clocks := make([]Clock, 0, len(rows))
	for _, row := range rows {
		occurredAt, errorValue := time.Parse(time.RFC3339, row.OccurredAt)
		if errorValue != nil {
			continue
		}
		clock := Clock{Kind: row.Kind, OccurredAt: occurredAt}
		if row.Member != nil {
			clock.Email = row.Member.Email
			clock.Name = row.Member.Name
		}
		clocks = append(clocks, clock)
	}
	return clocks, nil
}
