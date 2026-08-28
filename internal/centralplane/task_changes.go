package centralplane

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// A task as the central plane holds it, for carrying back to a device. People
// come back as addresses because that is what a device can turn into its own
// identifiers; member uuids mean nothing there.
type ChangedTask struct {
	CentralID        string
	DeviceTaskID     string
	Title            string
	Status           string
	Note             string
	Business         string
	Type             string
	Size             string
	StartsAt         string
	EndsAt           string
	UpdatedAt        string
	ParticipantMails []string
}

const changedTaskSelection = "id,title,status,note,business,type,size,starts_at,ends_at,updated_at,calendar,task_participant(member(email))"

// Calendar events live in the same table and carry the device's event uid as their
// mirror, so a mirror that read them would make a board task out of every meeting.
// TasksChangedSince answers the tasks written after that moment, oldest first, so
// a caller that keeps the last one it saw asks only for what it has not seen.
func (client *Client) TasksChangedSince(ctx context.Context, platform string, externalID string, since string, limit int) ([]ChangedTask, error) {
	session, errorValue := client.sessionFor(ctx, platform, externalID)
	if errorValue != nil {
		return nil, errorValue
	}
	query := url.Values{}
	query.Set("select", changedTaskSelection)
	query.Set("order", "updated_at.asc")
	query.Set("is_event", "eq.false")
	query.Set("limit", fmt.Sprintf("%d", limit))
	if strings.TrimSpace(since) != "" {
		query.Set("updated_at", "gt."+since)
	}

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
		return nil, fmt.Errorf("central plane refused to say what changed: %s", response.Status)
	}

	var rows []struct {
		ID        string `json:"id"`
		Title     string `json:"title"`
		Status    string `json:"status"`
		Note      string `json:"note"`
		Business  string `json:"business"`
		Type      string `json:"type"`
		Size      string `json:"size"`
		StartsAt  string `json:"starts_at"`
		EndsAt    string `json:"ends_at"`
		UpdatedAt string `json:"updated_at"`
		Calendar  struct {
			Mirrors []struct {
				Source     string `json:"source"`
				ExternalID string `json:"externalID"`
			} `json:"mirrors"`
		} `json:"calendar"`
		Participants []struct {
			Member struct {
				Email string `json:"email"`
			} `json:"member"`
		} `json:"task_participant"`
	}
	if errorValue := json.NewDecoder(response.Body).Decode(&rows); errorValue != nil {
		return nil, errorValue
	}

	changed := make([]ChangedTask, 0, len(rows))
	for _, row := range rows {
		task := ChangedTask{
			CentralID: row.ID,
			Title:     row.Title,
			Status:    row.Status,
			Note:      row.Note,
			Business:  row.Business,
			Type:      row.Type,
			Size:      row.Size,
			StartsAt:  row.StartsAt,
			EndsAt:    row.EndsAt,
			UpdatedAt: row.UpdatedAt,
		}
		for _, mirror := range row.Calendar.Mirrors {
			if mirror.Source == DeviceMirrorSource {
				task.DeviceTaskID = mirror.ExternalID
			}
		}
		for _, participant := range row.Participants {
			if address := strings.TrimSpace(participant.Member.Email); address != "" {
				task.ParticipantMails = append(task.ParticipantMails, address)
			}
		}
		changed = append(changed, task)
	}
	return changed, nil
}
