package admind

import (
	"fmt"
	"strings"
)

func validateCRMHTTPAccount(payload crmHTTPAccountPayload) error {
	if strings.TrimSpace(payload.Name) == "" || strings.TrimSpace(payload.OwnerPersonID) == "" {
		return fmt.Errorf("name and ownerPersonID are required")
	}
	if payload.Status != "" && !crmValueAllowed(payload.Status, "prospect", "active", "paused") {
		return fmt.Errorf("invalid account status")
	}
	if payload.Importance != "" && !crmValueAllowed(payload.Importance, "high", "medium", "low") {
		return fmt.Errorf("invalid account importance")
	}
	for _, accountType := range payload.Types {
		if !crmAccountTypes[accountType] {
			return fmt.Errorf("invalid account type")
		}
	}
	return nil
}

func validateCRMHTTPContact(payload crmHTTPContactPayload) error {
	if strings.TrimSpace(payload.Name) == "" || strings.TrimSpace(payload.OwnerPersonID) == "" {
		return fmt.Errorf("name and ownerPersonID are required")
	}
	if strings.TrimSpace(payload.Email) == "" && strings.TrimSpace(payload.Phone) == "" {
		return fmt.Errorf("email or phone is required")
	}
	return nil
}

func validateCRMHTTPActivity(payload crmHTTPActivityPayload, existingKind string) error {
	if strings.TrimSpace(payload.Title) == "" || strings.TrimSpace(payload.OccurredAt) == "" {
		return fmt.Errorf("title and occurredAt are required")
	}
	kind := strings.TrimSpace(payload.Kind)
	if kind == "stage_change" && existingKind != "stage_change" {
		return fmt.Errorf("invalid activity kind")
	}
	if kind != "stage_change" && !crmValueAllowed(kind, "note", "email", "meeting", "call", "task", "file", "event") {
		return fmt.Errorf("invalid activity kind")
	}
	if strings.TrimSpace(payload.AccountID) == "" && strings.TrimSpace(payload.ContactID) == "" && strings.TrimSpace(payload.OpportunityID) == "" {
		return fmt.Errorf("an account, contact, or opportunity is required")
	}
	return crmValidateTimestamp(strings.TrimSpace(payload.OccurredAt))
}
