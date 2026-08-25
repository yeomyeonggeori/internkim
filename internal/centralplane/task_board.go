package centralplane

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// A task as the company holds it, with the people on it as addresses so a
// device can turn them into whatever identifiers its own directory uses.
type BoardTask struct {
	CentralID        string
	ParentID         string
	Title            string
	Note             string
	Business         string
	Type             string
	Size             string
	Status           string
	StartsAt         string
	EndsAt           string
	DueAt            string
	CreatedAt        string
	UpdatedAt        string
	RequesterMail    string
	ParticipantMails []string
}

const boardTaskSelection = "id,parent_task_id,title,note,business,type,size,status,starts_at,ends_at,due_at,created_at,updated_at,requester:requester_id(email),task_participant(member(email))"

// BoardTasks answers the company's tasks, read as the person who asked so row
// level security decides what comes back. Events live in the same table and are
// left out, since a board is not a calendar.
func (client *Client) BoardTasks(ctx context.Context, platform string, externalID string) ([]BoardTask, error) {
	session, errorValue := client.sessionFor(ctx, platform, externalID)
	if errorValue != nil {
		return nil, errorValue
	}
	query := url.Values{}
	query.Set("select", boardTaskSelection)
	query.Set("order", "created_at.asc")
	query.Set("is_event", "eq.false")
	return client.boardTasksMatching(ctx, session, query)
}

func (client *Client) boardTasksMatching(ctx context.Context, session memberSession, query url.Values) ([]BoardTask, error) {
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
		return nil, fmt.Errorf("central plane refused to say what the board holds: %s", response.Status)
	}

	var rows []struct {
		ID        string `json:"id"`
		ParentID  any    `json:"parent_task_id"`
		Title     string `json:"title"`
		Note      any    `json:"note"`
		Business  any    `json:"business"`
		Type      any    `json:"type"`
		Size      any    `json:"size"`
		Status    string `json:"status"`
		StartsAt  any    `json:"starts_at"`
		EndsAt    any    `json:"ends_at"`
		DueAt     any    `json:"due_at"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
		Requester struct {
			Email string `json:"email"`
		} `json:"requester"`
		Participants []struct {
			Member struct {
				Email string `json:"email"`
			} `json:"member"`
		} `json:"task_participant"`
	}
	if errorValue := json.NewDecoder(response.Body).Decode(&rows); errorValue != nil {
		return nil, errorValue
	}

	tasks := make([]BoardTask, 0, len(rows))
	for _, row := range rows {
		task := BoardTask{
			CentralID:     row.ID,
			ParentID:      textOf(row.ParentID),
			Title:         row.Title,
			Note:          textOf(row.Note),
			Business:      textOf(row.Business),
			Type:          textOf(row.Type),
			Size:          textOf(row.Size),
			Status:        row.Status,
			StartsAt:      textOf(row.StartsAt),
			EndsAt:        textOf(row.EndsAt),
			DueAt:         textOf(row.DueAt),
			CreatedAt:     row.CreatedAt,
			UpdatedAt:     row.UpdatedAt,
			RequesterMail: strings.TrimSpace(row.Requester.Email),
		}
		for _, participant := range row.Participants {
			if email := strings.TrimSpace(participant.Member.Email); email != "" {
				task.ParticipantMails = append(task.ParticipantMails, email)
			}
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
}

func textOf(value any) string {
	held, isText := value.(string)
	if !isText {
		return ""
	}
	return strings.TrimSpace(held)
}
