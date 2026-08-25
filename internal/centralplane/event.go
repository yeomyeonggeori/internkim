package centralplane

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// An event as the company holds it. It is a task the company marked as one, so
// it lives in the same table and comes back with the people on it as addresses,
// which is what a device can turn into its own identifiers.
type Event struct {
	CentralID        string
	Title            string
	Note             string
	Location         string
	StartsAt         string
	EndsAt           string
	IsWholeDay       bool
	UpdatedAt        string
	ParticipantMails []string
	// ExpectedUpdatedAt is the version the writer read. The company refuses a
	// write whose version is already gone, so two people holding the same event
	// open no longer both win.
	ExpectedUpdatedAt string
}

// Postgres raises a serialization failure when a write names a version that is
// gone, and PostgREST carries that code through in the body it answers with.
const serializationFailureCode = "40001"

const eventSelection = "id,title,note,location,starts_at,ends_at,is_whole_day,updated_at,task_participant(member(email))"

// EventsBetween answers the company's events that overlap the window, earliest
// first. An event that started before it and has not ended overlaps it, so the
// comparison is against each end rather than each start.
func (client *Client) EventsBetween(ctx context.Context, platform string, externalID string, startISO string, endISO string) ([]Event, error) {
	session, errorValue := client.sessionFor(ctx, platform, externalID)
	if errorValue != nil {
		return nil, errorValue
	}
	query := url.Values{}
	query.Set("select", eventSelection)
	query.Set("order", "starts_at.asc")
	query.Set("is_event", "eq.true")
	if strings.TrimSpace(endISO) != "" {
		query.Set("starts_at", "lt."+endISO)
	}
	if strings.TrimSpace(startISO) != "" {
		query.Set("ends_at", "gte."+startISO)
	}
	return client.eventsMatching(ctx, session, query)
}

// A write answers with what the company now holds rather than with what the
// caller sent, so the reply carries the version the next write has to match.
func (client *Client) EventByID(ctx context.Context, platform string, externalID string, eventID string) (Event, bool, error) {
	session, errorValue := client.sessionFor(ctx, platform, externalID)
	if errorValue != nil {
		return Event{}, false, errorValue
	}
	query := url.Values{}
	query.Set("select", eventSelection)
	query.Set("is_event", "eq.true")
	query.Set("id", "eq."+eventID)
	events, errorValue := client.eventsMatching(ctx, session, query)
	if errorValue != nil || len(events) == 0 {
		return Event{}, false, errorValue
	}
	return events[0], true, nil
}

func (client *Client) eventsMatching(ctx context.Context, session memberSession, query url.Values) ([]Event, error) {
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet,
		strings.TrimSuffix(client.settings.ProjectURL, "/")+"/rest/v1/task?"+query.Encode(), nil)
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
		return nil, fmt.Errorf("central plane refused to say what is on the calendar: %s", response.Status)
	}

	var rows []struct {
		ID           string `json:"id"`
		Title        string `json:"title"`
		Note         string `json:"note"`
		Location     any    `json:"location"`
		StartsAt     string `json:"starts_at"`
		EndsAt       string `json:"ends_at"`
		IsWholeDay   bool   `json:"is_whole_day"`
		UpdatedAt    string `json:"updated_at"`
		Participants []struct {
			Member struct {
				Email string `json:"email"`
			} `json:"member"`
		} `json:"task_participant"`
	}
	if errorValue := json.NewDecoder(response.Body).Decode(&rows); errorValue != nil {
		return nil, errorValue
	}

	events := make([]Event, 0, len(rows))
	for _, row := range rows {
		event := Event{
			CentralID:  row.ID,
			Title:      row.Title,
			Note:       row.Note,
			Location:   locationName(row.Location),
			StartsAt:   row.StartsAt,
			EndsAt:     row.EndsAt,
			IsWholeDay: row.IsWholeDay,
			UpdatedAt:  row.UpdatedAt,
		}
		for _, participant := range row.Participants {
			if email := strings.TrimSpace(participant.Member.Email); email != "" {
				event.ParticipantMails = append(event.ParticipantMails, email)
			}
		}
		events = append(events, event)
	}
	return events, nil
}

// The column holds whatever the company app wrote there, which is an object with
// a name today and was a bare string before it.
func locationName(value any) string {
	switch held := value.(type) {
	case string:
		return strings.TrimSpace(held)
	case map[string]any:
		name, _ := held["name"].(string)
		return strings.TrimSpace(name)
	default:
		return ""
	}
}

// ErrEventVersionGone answers a write whose version the company no longer holds,
// which is somebody else having written the event in between.
var ErrEventVersionGone = errors.New("the event changed since it was read")

// SaveEvent writes an event as the person who asked for it, so the company's own
// rules decide whether they may. An empty CentralID makes a new one.
func (client *Client) SaveEvent(ctx context.Context, platform string, externalID string, event Event) (string, error) {
	session, errorValue := client.sessionFor(ctx, platform, externalID)
	if errorValue != nil {
		return "", errorValue
	}
	participants, errorValue := client.membersOf(ctx, session.memberID, event.ParticipantMails)
	if errorValue != nil {
		return "", errorValue
	}
	arguments := map[string]any{
		"target_task_id":             nullableString(event.CentralID),
		"target_title":               event.Title,
		"target_note":                nullableString(event.Note),
		"target_location":            nullableLocation(event.Location),
		"target_starts_at":           event.StartsAt,
		"target_ends_at":             event.EndsAt,
		"target_is_whole_day":        event.IsWholeDay,
		"target_size":                "M",
		"target_participant_ids":     participants,
		"target_expected_updated_at": nullableString(event.ExpectedUpdatedAt),
	}
	var savedID string
	if errorValue := client.callAsMember(ctx, session, "save_calendar_event", arguments, &savedID); errorValue != nil {
		if strings.Contains(errorValue.Error(), serializationFailureCode) {
			return "", ErrEventVersionGone
		}
		return "", errorValue
	}
	return savedID, nil
}

// DeleteEvent removes the event the company holds, as the person who may.
func (client *Client) DeleteEvent(ctx context.Context, platform string, externalID string, centralID string) error {
	session, errorValue := client.sessionFor(ctx, platform, externalID)
	if errorValue != nil {
		return errorValue
	}
	query := url.Values{}
	query.Set("id", "eq."+centralID)
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodDelete,
		strings.TrimSuffix(client.settings.ProjectURL, "/")+"/rest/v1/task?"+query.Encode(), nil)
	if errorValue != nil {
		return errorValue
	}
	client.signAsMember(request, session)
	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return fmt.Errorf("central plane refused to remove the event: %s", response.Status)
	}
	return nil
}

// The column holds an object the company app can name, so a device that knows
// only a line of text writes it as that object's name.
func nullableLocation(location string) any {
	trimmed := strings.TrimSpace(location)
	if trimmed == "" {
		return nil
	}
	return map[string]any{"name": trimmed}
}
