package admind

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

func (service *Service) writeCRMActivity(ctx context.Context, activity crmActivity) (crmActivity, error) {
	activity.ID = strings.TrimSpace(activity.ID)
	if activity.ID == "" {
		activity.ID = newCRMID("activity")
	}
	activity.Title = strings.TrimSpace(activity.Title)
	if activity.Title == "" {
		return crmActivity{}, fmt.Errorf("CRM activity title is required")
	}
	if activity.AccountID == "" && activity.ContactID == "" && activity.OpportunityID == "" {
		return crmActivity{}, fmt.Errorf("CRM activity requires an account, contact, or opportunity")
	}
	if errorValue := crmValidateTimestamp(activity.OccurredAt); errorValue != nil {
		return crmActivity{}, fmt.Errorf("invalid CRM activity occurrence time: %w", errorValue)
	}
	audit, errorValue := crmNormalizeAudit(activity.Audit)
	if errorValue != nil {
		return crmActivity{}, errorValue
	}
	activity.Audit = audit
	database, errorValue := service.openCRMDatabase(ctx)
	if errorValue != nil {
		return crmActivity{}, errorValue
	}
	defer database.Close()
	var existingKind string
	errorValue = database.QueryRowContext(ctx, "SELECT kind FROM activity WHERE id = ?", activity.ID).Scan(&existingKind)
	if errorValue != nil && errorValue != sql.ErrNoRows {
		return crmActivity{}, fmt.Errorf("read existing CRM activity %s: %w", activity.ID, errorValue)
	}
	if activity.Kind == "stage_change" {
		if errorValue == sql.ErrNoRows || existingKind != "stage_change" {
			return crmActivity{}, fmt.Errorf("CRM stage change activities are system generated")
		}
	} else if !crmValueAllowed(activity.Kind, "note", "email", "meeting", "call", "task", "file", "event") {
		return crmActivity{}, fmt.Errorf("invalid CRM activity kind %q", activity.Kind)
	}
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return crmActivity{}, errorValue
	}
	if errorValue := upsertCRMActivityInTransaction(ctx, transaction, activity); errorValue != nil {
		_ = transaction.Rollback()
		return crmActivity{}, errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return crmActivity{}, fmt.Errorf("commit CRM activity %s: %w", activity.ID, errorValue)
	}
	return service.readCRMActivity(ctx, activity.ID, true)
}

func insertCRMActivityInTransaction(ctx context.Context, transaction *sql.Tx, activity crmActivity) error {
	_, errorValue := transaction.ExecContext(ctx, `
INSERT INTO activity(
	id, account_id, contact_id, opportunity_id, business, kind, title, occurred_at,
	content, created_at, created_by_person_id, updated_at, updated_by_person_id,
	archived_at, archived_by_person_id
) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		activity.ID, crmNullableString(activity.AccountID), crmNullableString(activity.ContactID),
		crmNullableString(activity.OpportunityID), crmNullableString(activity.Business), activity.Kind,
		activity.Title, activity.OccurredAt, crmNullableString(activity.Content), activity.Audit.CreatedAt,
		activity.Audit.CreatedByPersonID, activity.Audit.UpdatedAt, activity.Audit.UpdatedByPersonID,
		crmNullableString(activity.Audit.ArchivedAt), crmNullableString(activity.Audit.ArchivedByPersonID))
	if errorValue != nil {
		return fmt.Errorf("insert CRM activity %s: %w", activity.ID, errorValue)
	}
	return nil
}

func upsertCRMActivityInTransaction(ctx context.Context, transaction *sql.Tx, activity crmActivity) error {
	_, errorValue := transaction.ExecContext(ctx, `
INSERT INTO activity(
	id, account_id, contact_id, opportunity_id, business, kind, title, occurred_at,
	content, created_at, created_by_person_id, updated_at, updated_by_person_id,
	archived_at, archived_by_person_id
) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	account_id = excluded.account_id,
	contact_id = excluded.contact_id,
	opportunity_id = excluded.opportunity_id,
	business = excluded.business,
	kind = excluded.kind,
	title = excluded.title,
	occurred_at = excluded.occurred_at,
	content = excluded.content,
	updated_at = excluded.updated_at,
	updated_by_person_id = excluded.updated_by_person_id,
	archived_at = excluded.archived_at,
	archived_by_person_id = excluded.archived_by_person_id`,
		activity.ID, crmNullableString(activity.AccountID), crmNullableString(activity.ContactID),
		crmNullableString(activity.OpportunityID), crmNullableString(activity.Business), activity.Kind,
		activity.Title, activity.OccurredAt, crmNullableString(activity.Content), activity.Audit.CreatedAt,
		activity.Audit.CreatedByPersonID, activity.Audit.UpdatedAt, activity.Audit.UpdatedByPersonID,
		crmNullableString(activity.Audit.ArchivedAt), crmNullableString(activity.Audit.ArchivedByPersonID))
	if errorValue != nil {
		return fmt.Errorf("write CRM activity %s: %w", activity.ID, errorValue)
	}
	return nil
}

func (service *Service) readCRMActivity(ctx context.Context, activityID string, includeArchived bool) (crmActivity, error) {
	database, errorValue := service.openCRMDatabase(ctx)
	if errorValue != nil {
		return crmActivity{}, errorValue
	}
	defer database.Close()
	activity, errorValue := scanCRMActivity(database.QueryRowContext(ctx, `
SELECT id, account_id, contact_id, opportunity_id, business, kind, title, occurred_at,
	content, created_at, created_by_person_id, updated_at, updated_by_person_id,
	archived_at, archived_by_person_id
FROM activity
WHERE id = ? AND (? = 1 OR archived_at IS NULL)`, strings.TrimSpace(activityID), crmBooleanInteger(includeArchived)))
	if errorValue == sql.ErrNoRows {
		return crmActivity{}, errCRMRecordNotFound
	}
	if errorValue != nil {
		return crmActivity{}, fmt.Errorf("read CRM activity %s: %w", activityID, errorValue)
	}
	return activity, nil
}

func (service *Service) listCRMActivities(ctx context.Context, includeArchived bool) ([]crmActivity, error) {
	database, errorValue := service.openCRMDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT id, account_id, contact_id, opportunity_id, business, kind, title, occurred_at,
	content, created_at, created_by_person_id, updated_at, updated_by_person_id,
	archived_at, archived_by_person_id
FROM activity
WHERE ? = 1 OR archived_at IS NULL
ORDER BY occurred_at DESC, id`, crmBooleanInteger(includeArchived))
	if errorValue != nil {
		return nil, fmt.Errorf("list CRM activities: %w", errorValue)
	}
	defer rows.Close()
	activities := []crmActivity{}
	for rows.Next() {
		activity, errorValue := scanCRMActivity(rows)
		if errorValue != nil {
			return nil, fmt.Errorf("scan CRM activity: %w", errorValue)
		}
		activities = append(activities, activity)
	}
	if errorValue := rows.Err(); errorValue != nil {
		return nil, fmt.Errorf("iterate CRM activities: %w", errorValue)
	}
	return activities, nil
}

func (service *Service) archiveCRMActivity(ctx context.Context, activityID string, archivedAt string, actorPersonID string) error {
	if errorValue := crmValidateTimestamp(archivedAt); errorValue != nil {
		return fmt.Errorf("invalid CRM activity archive time: %w", errorValue)
	}
	actorPersonID = strings.TrimSpace(actorPersonID)
	if actorPersonID == "" {
		return fmt.Errorf("CRM activity archive actor is required")
	}
	database, errorValue := service.openCRMDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	result, errorValue := database.ExecContext(ctx, `
UPDATE activity
SET archived_at = ?, archived_by_person_id = ?, updated_at = ?, updated_by_person_id = ?
WHERE id = ?`, archivedAt, actorPersonID, archivedAt, actorPersonID, strings.TrimSpace(activityID))
	if errorValue != nil {
		return fmt.Errorf("archive CRM activity %s: %w", activityID, errorValue)
	}
	return crmRequireAffectedRecord(result)
}

func scanCRMActivity(scanner crmRowScanner) (crmActivity, error) {
	var activity crmActivity
	var accountID sql.NullString
	var contactID sql.NullString
	var opportunityID sql.NullString
	var business sql.NullString
	var content sql.NullString
	var archivedAt sql.NullString
	var archivedByPersonID sql.NullString
	errorValue := scanner.Scan(
		&activity.ID, &accountID, &contactID, &opportunityID, &business, &activity.Kind,
		&activity.Title, &activity.OccurredAt, &content, &activity.Audit.CreatedAt,
		&activity.Audit.CreatedByPersonID, &activity.Audit.UpdatedAt, &activity.Audit.UpdatedByPersonID,
		&archivedAt, &archivedByPersonID,
	)
	if errorValue != nil {
		return crmActivity{}, errorValue
	}
	activity.AccountID = crmStringFromNull(accountID)
	activity.ContactID = crmStringFromNull(contactID)
	activity.OpportunityID = crmStringFromNull(opportunityID)
	activity.Business = crmStringFromNull(business)
	activity.Content = crmStringFromNull(content)
	activity.Audit.ArchivedAt = crmStringFromNull(archivedAt)
	activity.Audit.ArchivedByPersonID = crmStringFromNull(archivedByPersonID)
	return activity, nil
}
