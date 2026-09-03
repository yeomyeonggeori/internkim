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

// A CRM row a device recorded before the record held one. The device's ids are
// text and the record's are uuids, so each carry answers the id it minted.
type CarriedCRMOrganization struct {
	Name        string
	Status      string
	Types       []string
	Tags        []string
	Importance  string
	OwnerID     string
	Address     string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	ArchivedAt  string
}

type CarriedCRMContact struct {
	OrganizationID string
	Name           string
	Email          string
	Phone          string
	Title          string
	Department     string
	Description    string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	ArchivedAt     string
}

type CarriedCRMOpportunity struct {
	OrganizationID   string
	ContactID        string
	Name             string
	Business         string
	PipelineID       string
	StageID          string
	StagePosition    int
	StageChangedAt   time.Time
	OwnerID          string
	AmountMinor      *int64
	CurrencyCode     string
	BaseAmountMinor  *int64
	BaseCurrencyCode string
	Importance       string
	DueAt            string
	DueTimeZone      string
	LostReasonID     string
	Description      string
	CreatedAt        time.Time
	UpdatedAt        time.Time
	ArchivedAt       string
}

// The record keeps a CRM activity as a task linked to what it is about, which
// is what crm_activity_save and the stage-change trigger both write.
type CarriedCRMActivity struct {
	OrganizationID string
	OpportunityID  string
	ContactID      string
	Title          string
	Note           string
	Business       string
	Kind           string
	Status         string
	OccurredAt     string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (client *Client) CarryCRMOrganization(
	ctx context.Context,
	administratorEmail string,
	companyID string,
	organization CarriedCRMOrganization,
) (string, error) {
	body := map[string]any{
		"company_id": companyID,
		"name":       organization.Name,
		"status":     organization.Status,
		"types":      organization.Types,
		"tags":       organization.Tags,
		"importance": organization.Importance,
		"created_at": organization.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at": organization.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	addWhenWritten(body, "owner_id", organization.OwnerID)
	addWhenWritten(body, "address", organization.Address)
	addWhenWritten(body, "description", organization.Description)
	addWhenWritten(body, "archived_at", organization.ArchivedAt)
	return client.carryCRMRow(ctx, administratorEmail, "organization", body)
}

func (client *Client) CarryCRMContact(
	ctx context.Context,
	administratorEmail string,
	companyID string,
	contact CarriedCRMContact,
) (string, error) {
	body := map[string]any{
		"company_id": companyID,
		"name":       contact.Name,
		"created_at": contact.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at": contact.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	addWhenWritten(body, "organization_id", contact.OrganizationID)
	addWhenWritten(body, "email", contact.Email)
	addWhenWritten(body, "phone", contact.Phone)
	addWhenWritten(body, "title", contact.Title)
	addWhenWritten(body, "department", contact.Department)
	addWhenWritten(body, "description", contact.Description)
	addWhenWritten(body, "archived_at", contact.ArchivedAt)
	return client.carryCRMRow(ctx, administratorEmail, "contact", body)
}

func (client *Client) CarryCRMOpportunity(
	ctx context.Context,
	administratorEmail string,
	companyID string,
	opportunity CarriedCRMOpportunity,
) (string, error) {
	body := map[string]any{
		"company_id":       companyID,
		"organization_id":  opportunity.OrganizationID,
		"name":             opportunity.Name,
		"pipeline_id":      opportunity.PipelineID,
		"stage_id":         opportunity.StageID,
		"stage_position":   opportunity.StagePosition,
		"stage_changed_at": opportunity.StageChangedAt.UTC().Format(time.RFC3339Nano),
		"importance":       opportunity.Importance,
		"created_at":       opportunity.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at":       opportunity.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	addWhenWritten(body, "contact_id", opportunity.ContactID)
	addWhenWritten(body, "business", opportunity.Business)
	addWhenWritten(body, "owner_id", opportunity.OwnerID)
	addWhenWritten(body, "description", opportunity.Description)
	addWhenWritten(body, "lost_reason_id", opportunity.LostReasonID)
	addWhenWritten(body, "archived_at", opportunity.ArchivedAt)
	if opportunity.AmountMinor != nil {
		body["amount_minor"] = *opportunity.AmountMinor
		body["currency_code"] = opportunity.CurrencyCode
	}
	if opportunity.BaseAmountMinor != nil {
		body["base_amount_minor"] = *opportunity.BaseAmountMinor
		body["base_currency_code"] = opportunity.BaseCurrencyCode
	}
	if strings.TrimSpace(opportunity.DueAt) != "" {
		body["due_at"] = opportunity.DueAt
		body["due_time_zone"] = opportunity.DueTimeZone
	}
	return client.carryCRMRow(ctx, administratorEmail, "opportunity", body)
}

func (client *Client) CarryCRMActivity(
	ctx context.Context,
	administratorEmail string,
	companyID string,
	activity CarriedCRMActivity,
) (string, error) {
	body := map[string]any{
		"company_id": companyID,
		"title":      activity.Title,
		"type":       activity.Kind,
		"status":     activity.Status,
		"is_event":   false,
		"created_at": activity.CreatedAt.UTC().Format(time.RFC3339Nano),
		"updated_at": activity.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	addWhenWritten(body, "organization_id", activity.OrganizationID)
	addWhenWritten(body, "opportunity_id", activity.OpportunityID)
	addWhenWritten(body, "contact_id", activity.ContactID)
	addWhenWritten(body, "note", activity.Note)
	addWhenWritten(body, "business", activity.Business)
	addWhenWritten(body, "due_at", activity.OccurredAt)
	return client.carryCRMRow(ctx, administratorEmail, "task", body)
}

func addWhenWritten(body map[string]any, field string, value string) {
	if strings.TrimSpace(value) != "" {
		body[field] = value
	}
}

func (client *Client) carryCRMRow(
	ctx context.Context,
	signerEmail string,
	table string,
	body map[string]any,
) (string, error) {
	session, errorValue := client.sessionFor(ctx, "email", signerEmail)
	if errorValue != nil {
		return "", errorValue
	}
	payload, errorValue := json.Marshal(body)
	if errorValue != nil {
		return "", errorValue
	}
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimSuffix(client.settings.ProjectURL, "/")+"/rest/v1/"+table, bytes.NewReader(payload))
	if errorValue != nil {
		return "", errorValue
	}
	client.signAsMember(request, session)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Prefer", "return=representation")

	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return "", errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return "", fmt.Errorf("the record refused a carried %s row: %s", table, response.Status)
	}
	var written []struct {
		ID string `json:"id"`
	}
	if errorValue := json.NewDecoder(response.Body).Decode(&written); errorValue != nil {
		return "", errorValue
	}
	if len(written) == 0 || written[0].ID == "" {
		return "", fmt.Errorf("the record took a carried %s row and did not say what it is called", table)
	}
	return written[0].ID, nil
}
