package admind

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
)

func (service *Service) writeCRMOpportunity(ctx context.Context, opportunity crmOpportunity, contacts []crmOpportunityContact) (crmOpportunity, error) {
	opportunity.ID = strings.TrimSpace(opportunity.ID)
	if opportunity.ID == "" {
		opportunity.ID = newCRMID("opportunity")
	}
	opportunity.Name = strings.TrimSpace(opportunity.Name)
	opportunity.Pipeline = strings.TrimSpace(opportunity.Pipeline)
	opportunity.OwnerPersonID = strings.TrimSpace(opportunity.OwnerPersonID)
	if opportunity.Name == "" || opportunity.Pipeline == "" || opportunity.OwnerPersonID == "" {
		return crmOpportunity{}, fmt.Errorf("CRM opportunity name, pipeline, and owner are required")
	}
	if math.IsNaN(opportunity.StagePosition) || math.IsInf(opportunity.StagePosition, 0) {
		return crmOpportunity{}, fmt.Errorf("invalid CRM stage position")
	}
	if opportunity.CurrencyCode == "" {
		opportunity.CurrencyCode = "KRW"
	}
	if opportunity.Importance == "" {
		opportunity.Importance = "medium"
	}
	if !crmValueAllowed(opportunity.CurrencyCode, "KRW", "USD", "JPY", "EUR") {
		return crmOpportunity{}, fmt.Errorf("invalid CRM opportunity currency %q", opportunity.CurrencyCode)
	}
	if !crmValueAllowed(opportunity.Importance, "high", "medium", "low") {
		return crmOpportunity{}, fmt.Errorf("invalid CRM opportunity importance %q", opportunity.Importance)
	}
	if errorValue := crmValidateDueTime(opportunity.DueAt, opportunity.DueTimeZone); errorValue != nil {
		return crmOpportunity{}, errorValue
	}
	audit, errorValue := crmNormalizeAudit(opportunity.Audit)
	if errorValue != nil {
		return crmOpportunity{}, errorValue
	}
	opportunity.Audit = audit
	if opportunity.StageChangedAt == "" {
		opportunity.StageChangedAt = opportunity.Audit.CreatedAt
	}
	if errorValue := crmValidateTimestamp(opportunity.StageChangedAt); errorValue != nil {
		return crmOpportunity{}, fmt.Errorf("invalid CRM opportunity stage change time: %w", errorValue)
	}
	if errorValue := validateCRMOpportunityContacts(contacts); errorValue != nil {
		return crmOpportunity{}, errorValue
	}

	database, errorValue := service.openCRMDatabase(ctx)
	if errorValue != nil {
		return crmOpportunity{}, errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return crmOpportunity{}, fmt.Errorf("begin CRM opportunity transaction: %w", errorValue)
	}
	if _, errorValue := transaction.ExecContext(ctx, "PRAGMA defer_foreign_keys=ON"); errorValue != nil {
		_ = transaction.Rollback()
		return crmOpportunity{}, fmt.Errorf("defer CRM opportunity foreign keys: %w", errorValue)
	}
	var existingCount int
	if errorValue := transaction.QueryRowContext(ctx, "SELECT COUNT(*) FROM opportunity WHERE id = ?", opportunity.ID).Scan(&existingCount); errorValue != nil {
		_ = transaction.Rollback()
		return crmOpportunity{}, fmt.Errorf("check CRM opportunity %s: %w", opportunity.ID, errorValue)
	}
	if existingCount == 0 && opportunity.AccountID == "" && len(contacts) == 0 {
		_ = transaction.Rollback()
		return crmOpportunity{}, fmt.Errorf("CRM opportunity requires an account or contact")
	}
	if existingCount > 0 && opportunity.AccountID == "" && contacts != nil && len(contacts) == 0 {
		_ = transaction.Rollback()
		return crmOpportunity{}, fmt.Errorf("CRM opportunity requires an account or contact")
	}
	var existingAccountID string
	if existingCount > 0 {
		var existingAccountValue sql.NullString
		var existingPipeline string
		var existingStage string
		var existingStagePosition float64
		var existingStageChangedAt string
		if errorValue := transaction.QueryRowContext(ctx, `
SELECT account_id, pipeline, stage, stage_position, stage_changed_at
FROM opportunity
WHERE id = ?`, opportunity.ID).Scan(&existingAccountValue, &existingPipeline, &existingStage, &existingStagePosition, &existingStageChangedAt); errorValue != nil {
			_ = transaction.Rollback()
			return crmOpportunity{}, fmt.Errorf("read CRM opportunity stage %s: %w", opportunity.ID, errorValue)
		}
		existingAccountID = crmStringFromNull(existingAccountValue)
		if (existingAccountID == "") != (opportunity.AccountID == "") {
			_ = transaction.Rollback()
			return crmOpportunity{}, fmt.Errorf("CRM opportunity cannot change between account and contact-only customers")
		}
		if opportunity.Pipeline != existingPipeline || (opportunity.Stage != "" && opportunity.Stage != existingStage) {
			_ = transaction.Rollback()
			return crmOpportunity{}, fmt.Errorf("CRM opportunity stage changes require transitionCRMOpportunityStage")
		}
		opportunity.Stage = existingStage
		opportunity.StagePosition = existingStagePosition
		opportunity.StageChangedAt = existingStageChangedAt
		if existingAccountID != opportunity.AccountID && contacts == nil {
			_ = transaction.Rollback()
			return crmOpportunity{}, fmt.Errorf("CRM opportunity account changes require an explicit contact list")
		}
	}
	if opportunity.Stage == "" {
		if errorValue := transaction.QueryRowContext(ctx, `
SELECT stage
FROM pipeline_stage
WHERE pipeline = ?
ORDER BY position
LIMIT 1`, opportunity.Pipeline).Scan(&opportunity.Stage); errorValue != nil {
			_ = transaction.Rollback()
			return crmOpportunity{}, fmt.Errorf("resolve first CRM stage for %s: %w", opportunity.Pipeline, errorValue)
		}
	}
	if opportunity.StagePosition == 0 {
		if errorValue := transaction.QueryRowContext(ctx, `
SELECT COALESCE(MIN(stage_position) - 1024.0, 1024.0)
FROM opportunity
WHERE pipeline = ? AND stage = ?`, opportunity.Pipeline, opportunity.Stage).Scan(&opportunity.StagePosition); errorValue != nil {
			_ = transaction.Rollback()
			return crmOpportunity{}, fmt.Errorf("resolve CRM stage position: %w", errorValue)
		}
	}
	accountChanged := existingCount > 0 && existingAccountID != opportunity.AccountID
	contactsWrittenBeforeOpportunity := existingCount == 0 && opportunity.AccountID == ""
	if accountChanged && existingAccountID != "" && opportunity.AccountID != "" {
		if errorValue := replaceCRMOpportunityContactsInTransaction(ctx, transaction, opportunity.ID, []crmOpportunityContact{}); errorValue != nil {
			_ = transaction.Rollback()
			return crmOpportunity{}, errorValue
		}
	}
	if contactsWrittenBeforeOpportunity {
		if errorValue := replaceCRMOpportunityContactsInTransaction(ctx, transaction, opportunity.ID, contacts); errorValue != nil {
			_ = transaction.Rollback()
			return crmOpportunity{}, errorValue
		}
	}
	if errorValue := writeCRMOpportunityInTransaction(ctx, transaction, opportunity); errorValue != nil {
		_ = transaction.Rollback()
		return crmOpportunity{}, errorValue
	}
	if contacts != nil && !contactsWrittenBeforeOpportunity {
		if errorValue := replaceCRMOpportunityContactsInTransaction(ctx, transaction, opportunity.ID, contacts); errorValue != nil {
			_ = transaction.Rollback()
			return crmOpportunity{}, errorValue
		}
	}
	if existingCount == 0 {
		if errorValue := rebalanceCRMStagePositionsOnCollision(ctx, transaction, opportunity.Pipeline, opportunity.Stage, opportunity.ID, opportunity.StagePosition, "", opportunity.Audit.UpdatedAt, opportunity.Audit.UpdatedByPersonID); errorValue != nil {
			_ = transaction.Rollback()
			return crmOpportunity{}, errorValue
		}
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return crmOpportunity{}, fmt.Errorf("commit CRM opportunity %s: %w", opportunity.ID, errorValue)
	}
	written, _, errorValue := service.readCRMOpportunity(ctx, opportunity.ID, true)
	return written, errorValue
}

func writeCRMOpportunityInTransaction(ctx context.Context, transaction *sql.Tx, opportunity crmOpportunity) error {
	_, errorValue := transaction.ExecContext(ctx, `
INSERT INTO opportunity(
	id, account_id, business, name, pipeline, stage, stage_position, stage_changed_at,
	owner_person_id, owner_circle_id, amount_minor, currency_code, base_amount_minor,
	base_currency_code, importance, due_at, due_time_zone, lost_reason, description,
	created_at, created_by_person_id, updated_at, updated_by_person_id, archived_at,
	archived_by_person_id
) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	account_id = excluded.account_id,
	business = excluded.business,
	name = excluded.name,
	pipeline = excluded.pipeline,
	stage = excluded.stage,
	stage_position = excluded.stage_position,
	stage_changed_at = excluded.stage_changed_at,
	owner_person_id = excluded.owner_person_id,
	owner_circle_id = excluded.owner_circle_id,
	amount_minor = excluded.amount_minor,
	currency_code = excluded.currency_code,
	base_amount_minor = excluded.base_amount_minor,
	base_currency_code = excluded.base_currency_code,
	importance = excluded.importance,
	due_at = excluded.due_at,
	due_time_zone = excluded.due_time_zone,
	lost_reason = excluded.lost_reason,
	description = excluded.description,
	updated_at = excluded.updated_at,
	updated_by_person_id = excluded.updated_by_person_id,
	archived_at = excluded.archived_at,
	archived_by_person_id = excluded.archived_by_person_id`,
		opportunity.ID, crmNullableString(opportunity.AccountID), crmNullableString(opportunity.Business),
		opportunity.Name, opportunity.Pipeline, opportunity.Stage, opportunity.StagePosition,
		opportunity.StageChangedAt, opportunity.OwnerPersonID, crmNullableString(opportunity.OwnerCircleID),
		crmNullableInt64(opportunity.AmountMinor), opportunity.CurrencyCode, crmNullableInt64(opportunity.BaseAmountMinor),
		crmNullableString(opportunity.BaseCurrencyCode), opportunity.Importance, crmNullableString(opportunity.DueAt),
		crmNullableString(opportunity.DueTimeZone), crmNullableString(opportunity.LostReason),
		crmNullableString(opportunity.Description), opportunity.Audit.CreatedAt, opportunity.Audit.CreatedByPersonID,
		opportunity.Audit.UpdatedAt, opportunity.Audit.UpdatedByPersonID, crmNullableString(opportunity.Audit.ArchivedAt),
		crmNullableString(opportunity.Audit.ArchivedByPersonID))
	if errorValue != nil {
		return fmt.Errorf("write CRM opportunity %s: %w", opportunity.ID, errorValue)
	}
	return nil
}

func (service *Service) readCRMOpportunity(ctx context.Context, opportunityID string, includeArchived bool) (crmOpportunity, []crmOpportunityContact, error) {
	database, errorValue := service.openCRMDatabase(ctx)
	if errorValue != nil {
		return crmOpportunity{}, nil, errorValue
	}
	defer database.Close()
	opportunity, errorValue := scanCRMOpportunity(database.QueryRowContext(ctx, crmOpportunitySelectSQL+" WHERE id = ? AND (? = 1 OR archived_at IS NULL)", strings.TrimSpace(opportunityID), crmBooleanInteger(includeArchived)))
	if errorValue == sql.ErrNoRows {
		return crmOpportunity{}, nil, errCRMRecordNotFound
	}
	if errorValue != nil {
		return crmOpportunity{}, nil, fmt.Errorf("read CRM opportunity %s: %w", opportunityID, errorValue)
	}
	contacts, errorValue := readCRMOpportunityContacts(ctx, database, opportunity.ID)
	if errorValue != nil {
		return crmOpportunity{}, nil, errorValue
	}
	return opportunity, contacts, nil
}

func (service *Service) listCRMOpportunities(ctx context.Context, includeArchived bool) ([]crmOpportunity, error) {
	database, errorValue := service.openCRMDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, crmOpportunityListSQL, crmBooleanInteger(includeArchived))
	if errorValue != nil {
		return nil, fmt.Errorf("list CRM opportunities: %w", errorValue)
	}
	defer rows.Close()
	opportunities := []crmOpportunity{}
	for rows.Next() {
		opportunity, errorValue := scanCRMOpportunity(rows)
		if errorValue != nil {
			return nil, fmt.Errorf("scan CRM opportunity: %w", errorValue)
		}
		opportunities = append(opportunities, opportunity)
	}
	if errorValue := rows.Err(); errorValue != nil {
		return nil, fmt.Errorf("iterate CRM opportunities: %w", errorValue)
	}
	return opportunities, nil
}

const crmOpportunitySelectSQL = `
SELECT id, account_id, business, name, pipeline, stage, stage_position, stage_changed_at,
	owner_person_id, owner_circle_id, amount_minor, currency_code, base_amount_minor,
	base_currency_code, importance, due_at, due_time_zone, lost_reason, description,
	created_at, created_by_person_id, updated_at, updated_by_person_id, archived_at,
	archived_by_person_id
FROM opportunity`

const crmOpportunityListSQL = `
SELECT id, account_id, business, name, pipeline, stage, stage_position, stage_changed_at,
	owner_person_id, owner_circle_id, amount_minor, currency_code, base_amount_minor,
	base_currency_code, importance, due_at, due_time_zone, lost_reason, description,
	created_at, created_by_person_id, updated_at, updated_by_person_id, archived_at,
	archived_by_person_id
FROM opportunity
WHERE ? = 1 OR archived_at IS NULL
ORDER BY pipeline, stage, stage_position, id`

func scanCRMOpportunity(scanner crmRowScanner) (crmOpportunity, error) {
	var opportunity crmOpportunity
	var accountID sql.NullString
	var business sql.NullString
	var ownerCircleID sql.NullString
	var amountMinor sql.NullInt64
	var baseAmountMinor sql.NullInt64
	var baseCurrencyCode sql.NullString
	var dueAt sql.NullString
	var dueTimeZone sql.NullString
	var lostReason sql.NullString
	var description sql.NullString
	var archivedAt sql.NullString
	var archivedByPersonID sql.NullString
	errorValue := scanner.Scan(
		&opportunity.ID, &accountID, &business, &opportunity.Name, &opportunity.Pipeline,
		&opportunity.Stage, &opportunity.StagePosition, &opportunity.StageChangedAt,
		&opportunity.OwnerPersonID, &ownerCircleID, &amountMinor, &opportunity.CurrencyCode,
		&baseAmountMinor, &baseCurrencyCode, &opportunity.Importance, &dueAt, &dueTimeZone,
		&lostReason, &description, &opportunity.Audit.CreatedAt, &opportunity.Audit.CreatedByPersonID,
		&opportunity.Audit.UpdatedAt, &opportunity.Audit.UpdatedByPersonID, &archivedAt,
		&archivedByPersonID,
	)
	if errorValue != nil {
		return crmOpportunity{}, errorValue
	}
	opportunity.AccountID = crmStringFromNull(accountID)
	opportunity.Business = crmStringFromNull(business)
	opportunity.OwnerCircleID = crmStringFromNull(ownerCircleID)
	opportunity.AmountMinor = crmInt64FromNull(amountMinor)
	opportunity.BaseAmountMinor = crmInt64FromNull(baseAmountMinor)
	opportunity.BaseCurrencyCode = crmStringFromNull(baseCurrencyCode)
	opportunity.DueAt = crmStringFromNull(dueAt)
	opportunity.DueTimeZone = crmStringFromNull(dueTimeZone)
	opportunity.LostReason = crmStringFromNull(lostReason)
	opportunity.Description = crmStringFromNull(description)
	opportunity.Audit.ArchivedAt = crmStringFromNull(archivedAt)
	opportunity.Audit.ArchivedByPersonID = crmStringFromNull(archivedByPersonID)
	return opportunity, nil
}

func readCRMOpportunityContacts(ctx context.Context, database *sql.DB, opportunityID string) ([]crmOpportunityContact, error) {
	rows, errorValue := database.QueryContext(ctx, `
SELECT contact_id, is_primary
FROM opportunity_contact
WHERE opportunity_id = ?
ORDER BY is_primary DESC, contact_id`, opportunityID)
	if errorValue != nil {
		return nil, fmt.Errorf("read CRM opportunity %s contacts: %w", opportunityID, errorValue)
	}
	defer rows.Close()
	contacts := []crmOpportunityContact{}
	for rows.Next() {
		var contact crmOpportunityContact
		var isPrimary int
		if errorValue := rows.Scan(&contact.ContactID, &isPrimary); errorValue != nil {
			return nil, errorValue
		}
		contact.IsPrimary = isPrimary == 1
		contacts = append(contacts, contact)
	}
	if errorValue := rows.Err(); errorValue != nil {
		return nil, errorValue
	}
	return contacts, nil
}

func (service *Service) archiveCRMOpportunity(ctx context.Context, opportunityID string, archivedAt string, actorPersonID string) error {
	if errorValue := crmValidateTimestamp(archivedAt); errorValue != nil {
		return fmt.Errorf("invalid CRM opportunity archive time: %w", errorValue)
	}
	actorPersonID = strings.TrimSpace(actorPersonID)
	if actorPersonID == "" {
		return fmt.Errorf("CRM opportunity archive actor is required")
	}
	database, errorValue := service.openCRMDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	result, errorValue := database.ExecContext(ctx, `
UPDATE opportunity
SET archived_at = ?, archived_by_person_id = ?, updated_at = ?, updated_by_person_id = ?
WHERE id = ?`, archivedAt, actorPersonID, archivedAt, actorPersonID, strings.TrimSpace(opportunityID))
	if errorValue != nil {
		return fmt.Errorf("archive CRM opportunity %s: %w", opportunityID, errorValue)
	}
	return crmRequireAffectedRecord(result)
}
