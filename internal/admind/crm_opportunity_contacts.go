package admind

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

func replaceCRMOpportunityContactsInTransaction(ctx context.Context, transaction *sql.Tx, opportunityID string, contacts []crmOpportunityContact) error {
	if _, errorValue := transaction.ExecContext(ctx, `
UPDATE opportunity_contact
SET is_primary = 0
WHERE opportunity_id = ? AND is_primary = 1`, opportunityID); errorValue != nil {
		return fmt.Errorf("clear CRM opportunity %s primary contact: %w", opportunityID, errorValue)
	}
	for _, contact := range contacts {
		if _, errorValue := transaction.ExecContext(ctx, `
INSERT INTO opportunity_contact(opportunity_id, contact_id, is_primary)
VALUES(?, ?, ?)
ON CONFLICT(opportunity_id, contact_id) DO UPDATE SET is_primary = excluded.is_primary`,
			opportunityID, contact.ContactID, crmBooleanInteger(contact.IsPrimary)); errorValue != nil {
			return fmt.Errorf("link CRM opportunity %s contact %s: %w", opportunityID, contact.ContactID, errorValue)
		}
	}
	if len(contacts) == 0 {
		if _, errorValue := transaction.ExecContext(ctx, "DELETE FROM opportunity_contact WHERE opportunity_id = ?", opportunityID); errorValue != nil {
			return fmt.Errorf("remove CRM opportunity %s contacts: %w", opportunityID, errorValue)
		}
		return nil
	}
	contactIDs := make([]string, 0, len(contacts))
	for _, contact := range contacts {
		contactIDs = append(contactIDs, contact.ContactID)
	}
	rows, errorValue := transaction.QueryContext(ctx, "SELECT contact_id FROM opportunity_contact WHERE opportunity_id = ?", opportunityID)
	if errorValue != nil {
		return fmt.Errorf("list CRM opportunity %s contacts: %w", opportunityID, errorValue)
	}
	existingIDs := []string{}
	for rows.Next() {
		var contactID string
		if errorValue := rows.Scan(&contactID); errorValue != nil {
			rows.Close()
			return errorValue
		}
		existingIDs = append(existingIDs, contactID)
	}
	if errorValue := rows.Close(); errorValue != nil {
		return errorValue
	}
	for _, existingID := range existingIDs {
		if !crmContainsString(contactIDs, existingID) {
			if _, errorValue := transaction.ExecContext(ctx, "DELETE FROM opportunity_contact WHERE opportunity_id = ? AND contact_id = ?", opportunityID, existingID); errorValue != nil {
				return fmt.Errorf("unlink CRM opportunity %s contact %s: %w", opportunityID, existingID, errorValue)
			}
		}
	}
	return nil
}

func validateCRMOpportunityContacts(contacts []crmOpportunityContact) error {
	seen := map[string]bool{}
	primaryCount := 0
	for _, contact := range contacts {
		contactID := strings.TrimSpace(contact.ContactID)
		if contactID == "" {
			return fmt.Errorf("CRM opportunity contact ID is required")
		}
		if seen[contactID] {
			return fmt.Errorf("duplicate CRM opportunity contact %s", contactID)
		}
		seen[contactID] = true
		if contact.IsPrimary {
			primaryCount++
		}
	}
	if primaryCount > 1 {
		return fmt.Errorf("CRM opportunity can have only one primary contact")
	}
	return nil
}

func crmContainsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
