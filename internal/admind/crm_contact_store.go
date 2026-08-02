package admind

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

func (service *Service) writeCRMContact(ctx context.Context, contact crmContact) (crmContact, error) {
	contact.ID = strings.TrimSpace(contact.ID)
	if contact.ID == "" {
		contact.ID = newCRMID("contact")
	}
	contact.Name = strings.TrimSpace(contact.Name)
	contact.Email = strings.TrimSpace(contact.Email)
	contact.Phone = strings.TrimSpace(contact.Phone)
	contact.OwnerPersonID = strings.TrimSpace(contact.OwnerPersonID)
	if contact.Name == "" || contact.OwnerPersonID == "" {
		return crmContact{}, fmt.Errorf("CRM contact name and owner are required")
	}
	if contact.Email == "" && contact.Phone == "" {
		return crmContact{}, fmt.Errorf("CRM contact email or phone is required")
	}
	audit, errorValue := crmNormalizeAudit(contact.Audit)
	if errorValue != nil {
		return crmContact{}, errorValue
	}
	contact.Audit = audit
	database, errorValue := service.openCRMDatabase(ctx)
	if errorValue != nil {
		return crmContact{}, errorValue
	}
	defer database.Close()
	_, errorValue = database.ExecContext(ctx, `
INSERT INTO contact(
	id, account_id, name, email, phone, title, department, is_primary,
	owner_person_id, owner_circle_id, description, created_at, created_by_person_id,
	updated_at, updated_by_person_id, archived_at, archived_by_person_id
) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	account_id = excluded.account_id,
	name = excluded.name,
	email = excluded.email,
	phone = excluded.phone,
	title = excluded.title,
	department = excluded.department,
	is_primary = excluded.is_primary,
	owner_person_id = excluded.owner_person_id,
	owner_circle_id = excluded.owner_circle_id,
	description = excluded.description,
	updated_at = excluded.updated_at,
	updated_by_person_id = excluded.updated_by_person_id,
	archived_at = excluded.archived_at,
	archived_by_person_id = excluded.archived_by_person_id`,
		contact.ID, crmNullableString(contact.AccountID), contact.Name, crmNullableString(contact.Email),
		crmNullableString(contact.Phone), crmNullableString(contact.Title), crmNullableString(contact.Department),
		crmBooleanInteger(contact.IsPrimary), contact.OwnerPersonID, crmNullableString(contact.OwnerCircleID),
		crmNullableString(contact.Description), contact.Audit.CreatedAt, contact.Audit.CreatedByPersonID,
		contact.Audit.UpdatedAt, contact.Audit.UpdatedByPersonID, crmNullableString(contact.Audit.ArchivedAt),
		crmNullableString(contact.Audit.ArchivedByPersonID))
	if errorValue != nil {
		return crmContact{}, fmt.Errorf("write CRM contact %s: %w", contact.ID, errorValue)
	}
	return service.readCRMContact(ctx, contact.ID, true)
}

func (service *Service) readCRMContact(ctx context.Context, contactID string, includeArchived bool) (crmContact, error) {
	database, errorValue := service.openCRMDatabase(ctx)
	if errorValue != nil {
		return crmContact{}, errorValue
	}
	defer database.Close()
	row := database.QueryRowContext(ctx, `
SELECT id, account_id, name, email, phone, title, department, is_primary,
	owner_person_id, owner_circle_id, description, created_at, created_by_person_id,
	updated_at, updated_by_person_id, archived_at, archived_by_person_id
FROM contact
WHERE id = ? AND (? = 1 OR archived_at IS NULL)`, strings.TrimSpace(contactID), crmBooleanInteger(includeArchived))
	contact, errorValue := scanCRMContact(row)
	if errorValue == sql.ErrNoRows {
		return crmContact{}, errCRMRecordNotFound
	}
	if errorValue != nil {
		return crmContact{}, fmt.Errorf("read CRM contact %s: %w", contactID, errorValue)
	}
	return contact, nil
}

func (service *Service) listCRMContacts(ctx context.Context, includeArchived bool) ([]crmContact, error) {
	database, errorValue := service.openCRMDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT id, account_id, name, email, phone, title, department, is_primary,
	owner_person_id, owner_circle_id, description, created_at, created_by_person_id,
	updated_at, updated_by_person_id, archived_at, archived_by_person_id
FROM contact
WHERE ? = 1 OR archived_at IS NULL
ORDER BY lower(name), id`, crmBooleanInteger(includeArchived))
	if errorValue != nil {
		return nil, fmt.Errorf("list CRM contacts: %w", errorValue)
	}
	defer rows.Close()
	contacts := []crmContact{}
	for rows.Next() {
		contact, errorValue := scanCRMContact(rows)
		if errorValue != nil {
			return nil, fmt.Errorf("scan CRM contact: %w", errorValue)
		}
		contacts = append(contacts, contact)
	}
	if errorValue := rows.Err(); errorValue != nil {
		return nil, fmt.Errorf("iterate CRM contacts: %w", errorValue)
	}
	return contacts, nil
}

func (service *Service) archiveCRMContact(ctx context.Context, contactID string, archivedAt string, actorPersonID string) error {
	if errorValue := crmValidateTimestamp(archivedAt); errorValue != nil {
		return fmt.Errorf("invalid CRM contact archive time: %w", errorValue)
	}
	actorPersonID = strings.TrimSpace(actorPersonID)
	if actorPersonID == "" {
		return fmt.Errorf("CRM contact archive actor is required")
	}
	database, errorValue := service.openCRMDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	result, errorValue := database.ExecContext(ctx, `
UPDATE contact
SET archived_at = ?, archived_by_person_id = ?, updated_at = ?, updated_by_person_id = ?
WHERE id = ?`, archivedAt, actorPersonID, archivedAt, actorPersonID, strings.TrimSpace(contactID))
	if errorValue != nil {
		return fmt.Errorf("archive CRM contact %s: %w", contactID, errorValue)
	}
	return crmRequireAffectedRecord(result)
}

func scanCRMContact(scanner crmRowScanner) (crmContact, error) {
	var contact crmContact
	var accountID sql.NullString
	var email sql.NullString
	var phone sql.NullString
	var title sql.NullString
	var department sql.NullString
	var isPrimary int
	var ownerCircleID sql.NullString
	var description sql.NullString
	var archivedAt sql.NullString
	var archivedByPersonID sql.NullString
	errorValue := scanner.Scan(
		&contact.ID, &accountID, &contact.Name, &email, &phone, &title, &department, &isPrimary,
		&contact.OwnerPersonID, &ownerCircleID, &description, &contact.Audit.CreatedAt,
		&contact.Audit.CreatedByPersonID, &contact.Audit.UpdatedAt, &contact.Audit.UpdatedByPersonID,
		&archivedAt, &archivedByPersonID,
	)
	if errorValue != nil {
		return crmContact{}, errorValue
	}
	contact.AccountID = crmStringFromNull(accountID)
	contact.Email = crmStringFromNull(email)
	contact.Phone = crmStringFromNull(phone)
	contact.Title = crmStringFromNull(title)
	contact.Department = crmStringFromNull(department)
	contact.IsPrimary = isPrimary == 1
	contact.OwnerCircleID = crmStringFromNull(ownerCircleID)
	contact.Description = crmStringFromNull(description)
	contact.Audit.ArchivedAt = crmStringFromNull(archivedAt)
	contact.Audit.ArchivedByPersonID = crmStringFromNull(archivedByPersonID)
	return contact, nil
}
