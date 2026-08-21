package admind

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type crmPipeline struct {
	Pipeline  string
	Label     string
	Direction string
	IsActive  bool
}

func (service *Service) listCRMPipelines(ctx context.Context, activeOnly bool) ([]crmPipeline, error) {
	database, errorValue := service.openCRMDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT pipeline, label, direction, is_active
FROM pipeline
WHERE ? = 0 OR is_active = 1
ORDER BY pipeline`, crmBooleanInteger(activeOnly))
	if errorValue != nil {
		return nil, fmt.Errorf("list CRM pipelines: %w", errorValue)
	}
	defer rows.Close()
	pipelines := []crmPipeline{}
	for rows.Next() {
		var pipeline crmPipeline
		var isActive int
		if errorValue := rows.Scan(&pipeline.Pipeline, &pipeline.Label, &pipeline.Direction, &isActive); errorValue != nil {
			return nil, errorValue
		}
		pipeline.IsActive = isActive == 1
		pipelines = append(pipelines, pipeline)
	}
	if errorValue := rows.Err(); errorValue != nil {
		return nil, errorValue
	}
	return pipelines, nil
}

func (service *Service) writeCRMResourceLink(ctx context.Context, link crmResourceLink) (crmResourceLink, error) {
	link.ID = strings.TrimSpace(link.ID)
	if link.ID == "" {
		link.ID = newCRMID("resource")
	}
	link.EntityID = strings.TrimSpace(link.EntityID)
	link.ExternalResourceType = strings.TrimSpace(link.ExternalResourceType)
	link.ExternalResourceID = strings.TrimSpace(link.ExternalResourceID)
	link.CreatedByPersonID = strings.TrimSpace(link.CreatedByPersonID)
	if !crmValueAllowed(link.EntityType, "account", "contact", "opportunity", "activity") {
		return crmResourceLink{}, fmt.Errorf("invalid CRM resource entity type %q", link.EntityType)
	}
	if !crmValueAllowed(link.Service, "flow", "calendar", "mail", "files") {
		return crmResourceLink{}, fmt.Errorf("invalid CRM resource service %q", link.Service)
	}
	if link.EntityID == "" || link.ExternalResourceType == "" || link.ExternalResourceID == "" || link.CreatedByPersonID == "" {
		return crmResourceLink{}, fmt.Errorf("CRM resource link identifiers and actor are required")
	}
	if link.CreatedAt == "" {
		link.CreatedAt = crmCurrentTimestamp()
	}
	if errorValue := crmValidateTimestamp(link.CreatedAt); errorValue != nil {
		return crmResourceLink{}, fmt.Errorf("invalid CRM resource link creation time: %w", errorValue)
	}
	database, errorValue := service.openCRMDatabase(ctx)
	if errorValue != nil {
		return crmResourceLink{}, errorValue
	}
	defer database.Close()
	exists, errorValue := crmEntityExists(ctx, database, link.EntityType, link.EntityID)
	if errorValue != nil {
		return crmResourceLink{}, errorValue
	}
	if !exists {
		return crmResourceLink{}, errCRMRecordNotFound
	}
	_, errorValue = database.ExecContext(ctx, `
INSERT INTO resource_link(
	id, entity_type, entity_id, service, external_resource_type, external_resource_id,
	external_resource_url, created_at, created_by_person_id, removed_at, removed_by_person_id
) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		link.ID, link.EntityType, link.EntityID, link.Service, link.ExternalResourceType,
		link.ExternalResourceID, crmNullableString(link.ExternalResourceURL), link.CreatedAt,
		link.CreatedByPersonID, crmNullableString(link.RemovedAt), crmNullableString(link.RemovedByPersonID))
	if errorValue != nil {
		return crmResourceLink{}, fmt.Errorf("write CRM resource link %s: %w", link.ID, errorValue)
	}
	return link, nil
}

func (service *Service) listCRMResourceLinks(ctx context.Context, entityType string, entityID string, activeOnly bool) ([]crmResourceLink, error) {
	database, errorValue := service.openCRMDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	rows, errorValue := database.QueryContext(ctx, `
SELECT id, entity_type, entity_id, service, external_resource_type, external_resource_id,
	external_resource_url, created_at, created_by_person_id, removed_at, removed_by_person_id
FROM resource_link
WHERE entity_type = ? AND entity_id = ? AND (? = 0 OR removed_at IS NULL)
ORDER BY created_at, id`, entityType, entityID, crmBooleanInteger(activeOnly))
	if errorValue != nil {
		return nil, fmt.Errorf("list CRM resource links for %s/%s: %w", entityType, entityID, errorValue)
	}
	defer rows.Close()
	links := []crmResourceLink{}
	for rows.Next() {
		link, errorValue := scanCRMResourceLink(rows)
		if errorValue != nil {
			return nil, errorValue
		}
		links = append(links, link)
	}
	if errorValue := rows.Err(); errorValue != nil {
		return nil, errorValue
	}
	return links, nil
}

func (service *Service) removeCRMResourceLink(ctx context.Context, linkID string, removedAt string, actorPersonID string) error {
	if errorValue := crmValidateTimestamp(removedAt); errorValue != nil {
		return fmt.Errorf("invalid CRM resource removal time: %w", errorValue)
	}
	actorPersonID = strings.TrimSpace(actorPersonID)
	if actorPersonID == "" {
		return fmt.Errorf("CRM resource removal actor is required")
	}
	database, errorValue := service.openCRMDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	result, errorValue := database.ExecContext(ctx, `
UPDATE resource_link
SET removed_at = ?, removed_by_person_id = ?
WHERE id = ? AND removed_at IS NULL`, removedAt, actorPersonID, strings.TrimSpace(linkID))
	if errorValue != nil {
		return fmt.Errorf("remove CRM resource link %s: %w", linkID, errorValue)
	}
	return crmRequireAffectedRecord(result)
}

func crmEntityExists(ctx context.Context, database *sql.DB, entityType string, entityID string) (bool, error) {
	var count int
	var errorValue error
	switch entityType {
	case "account":
		errorValue = database.QueryRowContext(ctx, "SELECT COUNT(*) FROM account WHERE id = ?", entityID).Scan(&count)
	case "contact":
		errorValue = database.QueryRowContext(ctx, "SELECT COUNT(*) FROM contact WHERE id = ?", entityID).Scan(&count)
	case "opportunity":
		errorValue = database.QueryRowContext(ctx, "SELECT COUNT(*) FROM opportunity WHERE id = ?", entityID).Scan(&count)
	case "activity":
		errorValue = database.QueryRowContext(ctx, "SELECT COUNT(*) FROM activity WHERE id = ?", entityID).Scan(&count)
	default:
		return false, fmt.Errorf("invalid CRM entity type %q", entityType)
	}
	if errorValue != nil {
		return false, fmt.Errorf("check CRM entity %s/%s: %w", entityType, entityID, errorValue)
	}
	return count == 1, nil
}

func scanCRMResourceLink(scanner crmRowScanner) (crmResourceLink, error) {
	var link crmResourceLink
	var externalURL sql.NullString
	var removedAt sql.NullString
	var removedByPersonID sql.NullString
	errorValue := scanner.Scan(
		&link.ID, &link.EntityType, &link.EntityID, &link.Service, &link.ExternalResourceType,
		&link.ExternalResourceID, &externalURL, &link.CreatedAt, &link.CreatedByPersonID,
		&removedAt, &removedByPersonID,
	)
	if errorValue != nil {
		return crmResourceLink{}, errorValue
	}
	link.ExternalResourceURL = crmStringFromNull(externalURL)
	link.RemovedAt = crmStringFromNull(removedAt)
	link.RemovedByPersonID = crmStringFromNull(removedByPersonID)
	return link, nil
}
