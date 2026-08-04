package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
)

var crmAccountTypes = map[string]bool{
	"customer":  true,
	"partner":   true,
	"sponsor":   true,
	"vendor":    true,
	"investor":  true,
	"portfolio": true,
	"other":     true,
}

func (service *Service) writeCRMAccount(ctx context.Context, account crmAccount) (crmAccount, error) {
	account.ID = strings.TrimSpace(account.ID)
	if account.ID == "" {
		account.ID = newCRMID("account")
	}
	account.Name = strings.TrimSpace(account.Name)
	account.OwnerPersonID = strings.TrimSpace(account.OwnerPersonID)
	if account.Name == "" || account.OwnerPersonID == "" {
		return crmAccount{}, fmt.Errorf("CRM account name and owner are required")
	}
	if !crmValueAllowed(account.Status, "prospect", "active", "paused") {
		return crmAccount{}, fmt.Errorf("invalid CRM account status %q", account.Status)
	}
	if !crmValueAllowed(account.Importance, "high", "medium", "low") {
		return crmAccount{}, fmt.Errorf("invalid CRM account importance %q", account.Importance)
	}
	for _, accountType := range account.Types {
		if !crmAccountTypes[accountType] {
			return crmAccount{}, fmt.Errorf("invalid CRM account type %q", accountType)
		}
	}
	audit, errorValue := crmNormalizeAudit(account.Audit)
	if errorValue != nil {
		return crmAccount{}, errorValue
	}
	account.Audit = audit
	typesJSON, errorValue := crmOptionalStringArrayJSON(account.Types)
	if errorValue != nil {
		return crmAccount{}, fmt.Errorf("encode CRM account types: %w", errorValue)
	}
	tags := account.Tags
	if tags == nil {
		tags = []string{}
	}
	tagsJSON, errorValue := json.Marshal(tags)
	if errorValue != nil {
		return crmAccount{}, fmt.Errorf("encode CRM account tags: %w", errorValue)
	}
	database, errorValue := service.openCRMDatabase(ctx)
	if errorValue != nil {
		return crmAccount{}, errorValue
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx, `
INSERT INTO account(
	id, name, status, types, tags, importance, owner_person_id, owner_circle_id,
	address, description, created_at, created_by_person_id, updated_at,
	updated_by_person_id, archived_at, archived_by_person_id
) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	name = excluded.name,
	status = excluded.status,
	types = excluded.types,
	tags = excluded.tags,
	importance = excluded.importance,
	owner_person_id = excluded.owner_person_id,
	owner_circle_id = excluded.owner_circle_id,
	address = excluded.address,
	description = excluded.description,
	updated_at = excluded.updated_at,
	updated_by_person_id = excluded.updated_by_person_id,
	archived_at = excluded.archived_at,
	archived_by_person_id = excluded.archived_by_person_id`,
		account.ID, account.Name, account.Status, typesJSON, string(tagsJSON), account.Importance,
		account.OwnerPersonID, crmNullableString(account.OwnerCircleID), crmNullableString(account.Address),
		crmNullableString(account.Description), account.Audit.CreatedAt, account.Audit.CreatedByPersonID,
		account.Audit.UpdatedAt, account.Audit.UpdatedByPersonID, crmNullableString(account.Audit.ArchivedAt),
		crmNullableString(account.Audit.ArchivedByPersonID))
	if errorValue != nil {
		return crmAccount{}, fmt.Errorf("write CRM account %s: %w", account.ID, errorValue)
	}
	return service.readCRMAccount(ctx, account.ID, true)
}

func (service *Service) readCRMAccount(ctx context.Context, accountID string, includeArchived bool) (crmAccount, error) {
	database, errorValue := service.openCRMDatabase(ctx)
	if errorValue != nil {
		return crmAccount{}, errorValue
	}
	defer database.Close()
	row := database.QueryRowContext(ctx, `
SELECT id, name, status, types, tags, importance, owner_person_id, owner_circle_id,
	address, description, created_at, created_by_person_id, updated_at,
	updated_by_person_id, archived_at, archived_by_person_id
FROM account
WHERE id = ? AND (? = 1 OR archived_at IS NULL)`, strings.TrimSpace(accountID), crmBooleanInteger(includeArchived))
	account, errorValue := scanCRMAccount(row)
	if errorValue == sql.ErrNoRows {
		return crmAccount{}, errCRMRecordNotFound
	}
	if errorValue != nil {
		return crmAccount{}, fmt.Errorf("read CRM account %s: %w", accountID, errorValue)
	}
	return account, nil
}

func (service *Service) listCRMAccounts(ctx context.Context, includeArchived bool) ([]crmAccount, error) {
	database, errorValue := service.openCRMDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT id, name, status, types, tags, importance, owner_person_id, owner_circle_id,
	address, description, created_at, created_by_person_id, updated_at,
	updated_by_person_id, archived_at, archived_by_person_id
FROM account
WHERE ? = 1 OR archived_at IS NULL
ORDER BY lower(name), id`, crmBooleanInteger(includeArchived))
	if errorValue != nil {
		return nil, fmt.Errorf("list CRM accounts: %w", errorValue)
	}
	defer rows.Close()
	accounts := []crmAccount{}
	for rows.Next() {
		account, errorValue := scanCRMAccount(rows)
		if errorValue != nil {
			return nil, fmt.Errorf("scan CRM account: %w", errorValue)
		}
		accounts = append(accounts, account)
	}
	if errorValue := rows.Err(); errorValue != nil {
		return nil, fmt.Errorf("iterate CRM accounts: %w", errorValue)
	}
	return accounts, nil
}

func (service *Service) archiveCRMAccount(ctx context.Context, accountID string, archivedAt string, actorPersonID string) error {
	if errorValue := crmValidateTimestamp(archivedAt); errorValue != nil {
		return fmt.Errorf("invalid CRM account archive time: %w", errorValue)
	}
	actorPersonID = strings.TrimSpace(actorPersonID)
	if actorPersonID == "" {
		return fmt.Errorf("CRM account archive actor is required")
	}
	database, errorValue := service.openCRMDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	result, errorValue := database.ExecContext(ctx, `
UPDATE account
SET archived_at = ?, archived_by_person_id = ?, updated_at = ?, updated_by_person_id = ?
WHERE id = ?`, archivedAt, actorPersonID, archivedAt, actorPersonID, strings.TrimSpace(accountID))
	if errorValue != nil {
		return fmt.Errorf("archive CRM account %s: %w", accountID, errorValue)
	}
	return crmRequireAffectedRecord(result)
}

type crmRowScanner interface {
	Scan(...any) error
}

func scanCRMAccount(scanner crmRowScanner) (crmAccount, error) {
	var account crmAccount
	var typesJSON sql.NullString
	var tagsJSON string
	var ownerCircleID sql.NullString
	var address sql.NullString
	var description sql.NullString
	var archivedAt sql.NullString
	var archivedByPersonID sql.NullString
	errorValue := scanner.Scan(
		&account.ID, &account.Name, &account.Status, &typesJSON, &tagsJSON, &account.Importance,
		&account.OwnerPersonID, &ownerCircleID, &address, &description, &account.Audit.CreatedAt,
		&account.Audit.CreatedByPersonID, &account.Audit.UpdatedAt, &account.Audit.UpdatedByPersonID,
		&archivedAt, &archivedByPersonID,
	)
	if errorValue != nil {
		return crmAccount{}, errorValue
	}
	if typesJSON.Valid {
		if errorValue := json.Unmarshal([]byte(typesJSON.String), &account.Types); errorValue != nil {
			return crmAccount{}, errorValue
		}
	}
	if errorValue := json.Unmarshal([]byte(tagsJSON), &account.Tags); errorValue != nil {
		return crmAccount{}, errorValue
	}
	account.OwnerCircleID = crmStringFromNull(ownerCircleID)
	account.Address = crmStringFromNull(address)
	account.Description = crmStringFromNull(description)
	account.Audit.ArchivedAt = crmStringFromNull(archivedAt)
	account.Audit.ArchivedByPersonID = crmStringFromNull(archivedByPersonID)
	return account, nil
}

func crmOptionalStringArrayJSON(values []string) (any, error) {
	if values == nil {
		return nil, nil
	}
	encoded, errorValue := json.Marshal(values)
	if errorValue != nil {
		return nil, errorValue
	}
	return string(encoded), nil
}

func crmValueAllowed(value string, allowed ...string) bool {
	for _, allowedValue := range allowed {
		if value == allowedValue {
			return true
		}
	}
	return false
}

func crmBooleanInteger(value bool) int {
	if value {
		return 1
	}
	return 0
}

func crmRequireAffectedRecord(result sql.Result) error {
	affected, errorValue := result.RowsAffected()
	if errorValue != nil {
		return errorValue
	}
	if affected == 0 {
		return errCRMRecordNotFound
	}
	return nil
}
