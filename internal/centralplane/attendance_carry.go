package centralplane

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// A clock a device recorded before the company did, written as the person who
// made it so the record keeps whose it was.
type CarriedClock struct {
	Email      string
	Kind       string
	Location   string
	OccurredAt time.Time
}

// A leave a device recorded before the company did. Days is what it consumes,
// which is not the span it covers.
type CarriedLeave struct {
	Email      string
	Kind       string
	IsPaid     bool
	IsDeducted bool
	Days       float64
	Status     string
	StartsAt   time.Time
	EndsAt     time.Time
	Note       string
}

func (client *Client) CarryClock(ctx context.Context, clock CarriedClock) error {
	body := map[string]any{
		"kind":        clock.Kind,
		"occurred_at": clock.OccurredAt.UTC().Format(time.RFC3339Nano),
	}
	if strings.TrimSpace(clock.Location) != "" {
		body["location"] = clock.Location
	}
	return client.carry(ctx, clock.Email, "attendance", body)
}

// leave_requestable_by_owner admits only status 'requested' from the member;
// leave_recordable_by_admin admits any status from a company administrator.
func (client *Client) CarryLeave(ctx context.Context, administratorEmail string, leave CarriedLeave) error {
	body := map[string]any{
		"kind":        leave.Kind,
		"is_paid":     leave.IsPaid,
		"is_deducted": leave.IsDeducted,
		"days":        leave.Days,
		"status":      leave.Status,
		"starts_at":   leave.StartsAt.UTC().Format(time.RFC3339Nano),
		"ends_at":     leave.EndsAt.UTC().Format(time.RFC3339Nano),
	}
	if strings.TrimSpace(leave.Note) != "" {
		body["note"] = leave.Note
	}
	session, errorValue := client.sessionFor(ctx, "email", leave.Email)
	if errorValue != nil {
		return errorValue
	}
	body["member_id"] = session.memberID
	return client.write(ctx, administratorEmail, "leave", body)
}

// attendance_writable_by_owner is the only insert policy on that table.
func (client *Client) carry(ctx context.Context, email string, table string, body map[string]any) error {
	session, errorValue := client.sessionFor(ctx, "email", email)
	if errorValue != nil {
		return errorValue
	}
	body["member_id"] = session.memberID
	return client.write(ctx, email, table, body)
}

func (client *Client) write(ctx context.Context, signerEmail string, table string, body map[string]any) error {
	session, errorValue := client.sessionFor(ctx, "email", signerEmail)
	if errorValue != nil {
		return errorValue
	}
	payload, errorValue := json.Marshal(body)
	if errorValue != nil {
		return errorValue
	}
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimSuffix(client.settings.ProjectURL, "/")+"/rest/v1/"+table, bytes.NewReader(payload))
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
	if response.StatusCode >= 300 {
		return fmt.Errorf("the record refused a carried %s row: %s", table, response.Status)
	}
	return nil
}
