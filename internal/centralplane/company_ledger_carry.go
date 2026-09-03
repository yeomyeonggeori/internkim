package centralplane

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// A number, a dated event and a document a device recorded before the company
// did. Each keeps what it was written with rather than what a fresh call would
// mint: a metric its period, a document its number and the day it was issued.
type CarriedCompanyMetric struct {
	Metric       string
	Year         int
	Quarter      int
	Month        int
	Value        float64
	CurrencyCode string
	ValueUSD     *float64
	Unit         string
	Note         string
	UpdatedAt    time.Time
}

type CarriedCompanyRecord struct {
	Category   string
	RecordDate string
	Title      string
	Detail     string
	Attributes map[string]string
	UpdatedAt  time.Time
}

type CarriedCompanyDocument struct {
	RequesterEmail string
	DocumentNumber string
	Kind           string
	DocumentType   string
	Title          string
	Counterpart    string
	Language       string
	FilePath       string
	Summary        string
	IssuedAt       time.Time
}

// company_metric_written_by_admin and company_record_written_by_admin admit a
// company administrator, so a carry that names no person writes as the one who
// claimed the device.
func (client *Client) CarryCompanyMetric(
	ctx context.Context,
	administratorEmail string,
	companyID string,
	metric CarriedCompanyMetric,
) error {
	body := map[string]any{
		"company_id": companyID,
		"metric":     metric.Metric,
		"year":       metric.Year,
		"quarter":    metric.Quarter,
		"month":      metric.Month,
		"value":      metric.Value,
		"updated_at": metric.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	if strings.TrimSpace(metric.CurrencyCode) != "" {
		body["currency_code"] = metric.CurrencyCode
		body["value_usd"] = metric.ValueUSD
	}
	if strings.TrimSpace(metric.Unit) != "" {
		body["unit"] = metric.Unit
	}
	if strings.TrimSpace(metric.Note) != "" {
		body["note"] = metric.Note
	}
	return client.write(ctx, administratorEmail, "company_metric", body)
}

func (client *Client) CarryCompanyRecord(
	ctx context.Context,
	administratorEmail string,
	companyID string,
	record CarriedCompanyRecord,
) error {
	attributes := record.Attributes
	if attributes == nil {
		attributes = map[string]string{}
	}
	body := map[string]any{
		"company_id": companyID,
		"category":   record.Category,
		"title":      record.Title,
		"attributes": attributes,
		"updated_at": record.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
	if strings.TrimSpace(record.RecordDate) != "" {
		body["record_date"] = record.RecordDate
	}
	if strings.TrimSpace(record.Detail) != "" {
		body["detail"] = record.Detail
	}
	return client.write(ctx, administratorEmail, "company_record", body)
}

// company_document_written_by_colleague admits anybody who works here, and the
// paperwork skill ran as the person who asked, so the document is written as
// them and keeps them as its requester.
func (client *Client) CarryCompanyDocument(
	ctx context.Context,
	companyID string,
	document CarriedCompanyDocument,
) error {
	session, errorValue := client.sessionFor(ctx, "email", document.RequesterEmail)
	if errorValue != nil {
		return errorValue
	}
	body := map[string]any{
		"company_id":    companyID,
		"kind":          document.Kind,
		"document_type": document.DocumentType,
		"title":         document.Title,
		"requester_id":  session.memberID,
		"issued_at":     document.IssuedAt.UTC().Format(time.RFC3339Nano),
	}
	for field, value := range map[string]string{
		"document_number": document.DocumentNumber,
		"counterpart":     document.Counterpart,
		"language":        document.Language,
		"file_path":       document.FilePath,
		"summary":         document.Summary,
	} {
		if strings.TrimSpace(value) != "" {
			body[field] = value
		}
	}
	return client.writeAs(ctx, session, "company_document", body)
}

func (client *Client) CompanyRowID(ctx context.Context, signerEmail string) (string, error) {
	session, errorValue := client.sessionFor(ctx, "email", signerEmail)
	if errorValue != nil {
		return "", errorValue
	}
	request, errorValue := http.NewRequestWithContext(ctx, http.MethodGet,
		strings.TrimSuffix(client.settings.ProjectURL, "/")+"/rest/v1/company?select=id&limit=1", nil)
	if errorValue != nil {
		return "", errorValue
	}
	client.signAsMember(request, session)

	response, errorValue := client.httpClient.Do(request)
	if errorValue != nil {
		return "", errorValue
	}
	defer response.Body.Close()
	if response.StatusCode >= 300 {
		return "", fmt.Errorf("the record refused to name the company: %s", response.Status)
	}
	var rows []struct {
		ID string `json:"id"`
	}
	if errorValue := json.NewDecoder(response.Body).Decode(&rows); errorValue != nil {
		return "", errorValue
	}
	if len(rows) == 0 {
		return "", fmt.Errorf("the record names no company %s works for", signerEmail)
	}
	return rows[0].ID, nil
}
