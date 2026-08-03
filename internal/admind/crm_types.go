package admind

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

var errCRMRecordNotFound = errors.New("CRM record not found")

type crmAuditFields struct {
	CreatedAt          string
	CreatedByPersonID  string
	UpdatedAt          string
	UpdatedByPersonID  string
	ArchivedAt         string
	ArchivedByPersonID string
}

type crmAccount struct {
	ID            string
	Name          string
	Status        string
	Types         []string
	Tags          []string
	Importance    string
	OwnerPersonID string
	OwnerCircleID string
	Address       string
	Description   string
	Audit         crmAuditFields
}

type crmContact struct {
	ID            string
	AccountID     string
	Name          string
	Email         string
	Phone         string
	Title         string
	Department    string
	IsPrimary     bool
	OwnerPersonID string
	OwnerCircleID string
	Description   string
	Audit         crmAuditFields
}

type crmOpportunity struct {
	ID               string
	AccountID        string
	Business         string
	Name             string
	Pipeline         string
	Stage            string
	StagePosition    float64
	StageChangedAt   string
	OwnerPersonID    string
	OwnerCircleID    string
	AmountMinor      *int64
	CurrencyCode     string
	BaseAmountMinor  *int64
	BaseCurrencyCode string
	Importance       string
	DueAt            string
	DueTimeZone      string
	LostReason       string
	Description      string
	Audit            crmAuditFields
}

type crmOpportunityContact struct {
	ContactID string
	IsPrimary bool
}

type crmActivity struct {
	ID            string
	AccountID     string
	ContactID     string
	OpportunityID string
	Business      string
	Kind          string
	Title         string
	OccurredAt    string
	Content       string
	Audit         crmAuditFields
}

type crmResourceLink struct {
	ID                   string
	EntityType           string
	EntityID             string
	Service              string
	ExternalResourceType string
	ExternalResourceID   string
	ExternalResourceURL  string
	CreatedAt            string
	CreatedByPersonID    string
	RemovedAt            string
	RemovedByPersonID    string
}

type crmOpportunityStageTransition struct {
	OpportunityID       string
	Stage               string
	StagePosition       float64
	BeforeOpportunityID string
	OccurredAt          string
	ActorPersonID       string
	LostReason          string
	BaseAmountMinor     *int64
	BaseCurrencyCode    string
}

func newCRMID(prefix string) string {
	return prefix + "-" + randomHex(16)
}

func crmCurrentTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}

func crmNormalizeAudit(audit crmAuditFields) (crmAuditFields, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	if audit.CreatedAt == "" {
		audit.CreatedAt = now
	}
	if audit.UpdatedAt == "" {
		audit.UpdatedAt = audit.CreatedAt
	}
	if audit.CreatedByPersonID == "" {
		audit.CreatedByPersonID = audit.UpdatedByPersonID
	}
	if audit.UpdatedByPersonID == "" {
		audit.UpdatedByPersonID = audit.CreatedByPersonID
	}
	if errorValue := crmValidateTimestamp(audit.CreatedAt); errorValue != nil {
		return crmAuditFields{}, fmt.Errorf("invalid CRM created time: %w", errorValue)
	}
	if errorValue := crmValidateTimestamp(audit.UpdatedAt); errorValue != nil {
		return crmAuditFields{}, fmt.Errorf("invalid CRM updated time: %w", errorValue)
	}
	if audit.ArchivedAt != "" {
		if errorValue := crmValidateTimestamp(audit.ArchivedAt); errorValue != nil {
			return crmAuditFields{}, fmt.Errorf("invalid CRM archived time: %w", errorValue)
		}
		if strings.TrimSpace(audit.ArchivedByPersonID) == "" {
			return crmAuditFields{}, fmt.Errorf("CRM archived actor is required")
		}
	} else if strings.TrimSpace(audit.ArchivedByPersonID) != "" {
		return crmAuditFields{}, fmt.Errorf("CRM archived time is required")
	}
	if strings.TrimSpace(audit.CreatedByPersonID) == "" || strings.TrimSpace(audit.UpdatedByPersonID) == "" {
		return crmAuditFields{}, fmt.Errorf("CRM audit actors are required")
	}
	return audit, nil
}

func crmValidateTimestamp(value string) error {
	parsed, errorValue := time.Parse(time.RFC3339, value)
	if errorValue != nil {
		return errorValue
	}
	if parsed.UTC().Format(time.RFC3339) != value {
		return fmt.Errorf("time must use UTC RFC3339 seconds")
	}
	return nil
}

func crmValidateDueTime(dueAt string, dueTimeZone string) error {
	if (dueAt == "") != (dueTimeZone == "") {
		return fmt.Errorf("CRM due time and time zone must be provided together")
	}
	if dueAt == "" {
		return nil
	}
	if errorValue := crmValidateTimestamp(dueAt); errorValue != nil {
		return fmt.Errorf("invalid CRM due time: %w", errorValue)
	}
	if _, errorValue := time.LoadLocation(dueTimeZone); errorValue != nil {
		return fmt.Errorf("invalid CRM due time zone %q: %w", dueTimeZone, errorValue)
	}
	return nil
}

func crmNullableString(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func crmNullableInt64(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

func crmStringFromNull(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func crmInt64FromNull(value sql.NullInt64) *int64 {
	if !value.Valid {
		return nil
	}
	result := value.Int64
	return &result
}
