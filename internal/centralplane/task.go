package centralplane

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// A task the device holds, in the terms the central plane keeps. The actor is
// named by the messenger account the central plane can issue a session for;
// everyone else by the address the company knows them by. CentralID is what the
// central plane called the task when it last took it, and is empty the first
// time. DeviceTaskID is the device's own identifier, and is set only for a row
// being carried across.
type Task struct {
	CentralID        string
	DeviceTaskID     string
	ActorPlatform    string
	ActorExternalID  string
	Title            string
	Status           string
	Note             string
	Business         string
	Type             string
	Size             string
	StartsAt         string
	EndsAt           string
	WritesDates      bool
	ParticipantMails []string
}

// MemberOf answers which member the company knows at that address, or an empty
// string when nobody does. Somebody who has not been invited yet is an ordinary
// answer, not a failure.
func (client *Client) MemberOf(ctx context.Context, email string) (string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" {
		return "", nil
	}
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet,
		strings.TrimSuffix(client.settings.AppURL, "/")+"/api/agent/member?email="+url.QueryEscape(email), nil)
	if errorValue != nil {
		return "", errorValue
	}
	request.Header.Set("Authorization", "Bearer "+client.settings.AgentAPIKey)

	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return "", errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return "", fmt.Errorf("central plane refused a lookup of %s: %s", email, response.Status)
	}
	var answer struct {
		Member *struct {
			MemberID string `json:"memberID"`
		} `json:"member"`
	}
	if errorValue := json.NewDecoder(response.Body).Decode(&answer); errorValue != nil {
		return "", errorValue
	}
	if answer.Member == nil {
		return "", nil
	}
	return answer.Member.MemberID, nil
}

// SaveTask writes the task as the person it belongs to. task_save is granted
// to authenticated alone and resolves authority from the caller, so the device
// holds a member's token for the length of the call and never the service key.
func (client *Client) SaveTask(ctx context.Context, task Task) (string, error) {
	session, errorValue := client.sessionFor(ctx, task.ActorPlatform, task.ActorExternalID)
	if errorValue != nil {
		return "", errorValue
	}

	participants, errorValue := client.membersOf(ctx, session.memberID, task.ParticipantMails)
	if errorValue != nil {
		return "", errorValue
	}

	arguments := map[string]any{
		"target_task_id":         nullableString(task.CentralID),
		"target_title":           task.Title,
		"target_status":          task.Status,
		"target_note":            task.Note,
		"target_business":        nullableString(task.Business),
		"target_type":            nullableString(task.Type),
		"target_size":            nullableString(task.Size),
		"target_starts_at":       nullableString(task.StartsAt),
		"target_ends_at":         nullableString(task.EndsAt),
		"target_write_dates":     task.WritesDates,
		"target_participant_ids": participants,
		"target_mirrors":         deviceMirrors(task.DeviceTaskID),
	}
	var savedID string
	if errorValue := client.callAsMember(ctx, session, "task_save", arguments, &savedID); errorValue != nil {
		return "", errorValue
	}
	return savedID, nil
}

// DeleteTask removes the task the central plane holds, as the person who may.
func (client *Client) DeleteTask(ctx context.Context, platform string, externalID string, centralID string) error {
	if strings.TrimSpace(centralID) == "" {
		return nil
	}
	session, errorValue := client.sessionFor(ctx, platform, externalID)
	if errorValue != nil {
		return errorValue
	}
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodDelete,
		strings.TrimSuffix(client.settings.ProjectURL, "/")+"/rest/v1/task?id=eq."+url.QueryEscape(centralID), nil)
	if errorValue != nil {
		return errorValue
	}
	client.signAsMember(request, session)
	askForWhatWasRemoved(request)

	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return fmt.Errorf("central plane refused to remove task %s: %s", centralID, response.Status)
	}
	return confirmSomethingWasRemoved(response, "task "+centralID)
}

// A delete that row level security narrows to nothing still answers as though it
// worked, so the rows it removed are asked for and an empty answer is a refusal.
func askForWhatWasRemoved(request *http.Request) {
	request.Header.Set("Prefer", "return=representation")
}

func confirmSomethingWasRemoved(response *http.Response, subject string) error {
	var removed []json.RawMessage
	if errorValue := json.NewDecoder(response.Body).Decode(&removed); errorValue != nil {
		return fmt.Errorf("central plane did not say what it removed of %s: %w", subject, errorValue)
	}
	if len(removed) == 0 {
		return fmt.Errorf("central plane removed nothing of %s, which its permissions do not allow", subject)
	}
	return nil
}

// The actor is a participant of their own task, and task_save refuses anyone
// who is not a member here, so an address nobody is registered under is left out
// rather than failing the write.
// The attendee list is the one that was asked for. Seating the writer in every
// list meant somebody who took themselves off an event was put back by the same
// write that took them off, and the answer said it had worked.
//
// A write that names nobody at all still seats the writer, because a task with
// no participant and no requester is one only an admin could edit afterwards.
// Naming somebody the company does not know is not naming nobody: that list was
// asked for, and it stays as short as it resolved.
func (client *Client) membersOf(ctx context.Context, actorMemberID string, emails []string) ([]string, error) {
	if len(emails) == 0 && strings.TrimSpace(actorMemberID) != "" {
		return []string{actorMemberID}, nil
	}
	members := []string{}
	seen := map[string]bool{}
	for _, email := range emails {
		memberID, errorValue := client.MemberOf(ctx, email)
		if errorValue != nil {
			return nil, errorValue
		}
		if memberID == "" || seen[memberID] {
			continue
		}
		seen[memberID] = true
		members = append(members, memberID)
	}
	return members, nil
}

func (client *Client) callAsMember(ctx context.Context, session memberSession, procedure string, arguments map[string]any, answer any) error {
	payload, errorValue := json.Marshal(arguments)
	if errorValue != nil {
		return errorValue
	}
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimSuffix(client.settings.ProjectURL, "/")+"/rest/v1/rpc/"+procedure, bytes.NewReader(payload))
	if errorValue != nil {
		return errorValue
	}
	client.signAsMember(request, session)
	request.Header.Set("Content-Type", "application/json")

	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(response.Body, 4096))
	if response.StatusCode >= 300 {
		return fmt.Errorf("central plane refused %s: %s: %s", procedure, response.Status, strings.TrimSpace(string(body)))
	}
	return json.Unmarshal(body, answer)
}

func (client *Client) signAsMember(request *http.Request, session memberSession) {
	request.Header.Set("apikey", client.settings.PublishableKey)
	request.Header.Set("Authorization", "Bearer "+session.accessToken)
}

func nullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}
