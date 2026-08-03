package admind

import (
	"context"
	"fmt"
	"strings"
)

func (service *Service) restoreCRMRecord(ctx context.Context, entityType string, recordID string, restoredAt string, actorPersonID string) error {
	if errorValue := crmValidateTimestamp(restoredAt); errorValue != nil {
		return fmt.Errorf("invalid CRM restore time: %w", errorValue)
	}
	if strings.TrimSpace(recordID) == "" || strings.TrimSpace(actorPersonID) == "" {
		return fmt.Errorf("CRM restore record and actor are required")
	}
	database, errorValue := service.openCRMDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	query, errorValue := crmRestoreQuery(entityType)
	if errorValue != nil {
		return errorValue
	}
	result, errorValue := database.ExecContext(ctx, query, restoredAt, actorPersonID, strings.TrimSpace(recordID))
	if errorValue != nil {
		return fmt.Errorf("restore CRM %s %s: %w", entityType, recordID, errorValue)
	}
	return crmRequireAffectedRecord(result)
}

func crmRestoreQuery(entityType string) (string, error) {
	switch entityType {
	case "account":
		return `UPDATE account
SET archived_at = NULL, archived_by_person_id = NULL, updated_at = ?, updated_by_person_id = ?
WHERE id = ? AND archived_at IS NOT NULL`, nil
	case "contact":
		return `UPDATE contact
SET archived_at = NULL, archived_by_person_id = NULL, updated_at = ?, updated_by_person_id = ?
WHERE id = ? AND archived_at IS NOT NULL`, nil
	case "opportunity":
		return `UPDATE opportunity
SET archived_at = NULL, archived_by_person_id = NULL, updated_at = ?, updated_by_person_id = ?
WHERE id = ? AND archived_at IS NOT NULL`, nil
	case "activity":
		return `UPDATE activity
SET archived_at = NULL, archived_by_person_id = NULL, updated_at = ?, updated_by_person_id = ?
WHERE id = ? AND archived_at IS NOT NULL`, nil
	default:
		return "", fmt.Errorf("invalid CRM restore entity type %q", entityType)
	}
}
