package admind

import (
	"context"
	"database/sql"
)

const organizationGroupsInitializedKey = "groups_initialized"

func (service *Service) writeOrganizationGroups(ctx context.Context, groups []orgGroupRecord) error {
	normalizedGroups, groupAliases := normalizeOrganizationGroupsWithAliases(groups)
	if errorValue := validateOrganizationGroupHierarchy(normalizedGroups); errorValue != nil {
		return errorValue
	}
	database, errorValue := service.openOrganizationDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	previousGroups, errorValue := readOrganizationGroupsFromQueryRunner(ctx, transaction)
	if errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	changedGroupIDs := changedOrganizationGroupIDs(previousGroups, normalizedGroups, groupAliases)
	if errorValue := invalidateOrganizationGroups(ctx, transaction, changedGroupIDs); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if _, errorValue := transaction.ExecContext(ctx, "DELETE FROM organization_groups"); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	for index, group := range normalizedGroups {
		if _, errorValue := transaction.ExecContext(ctx, "INSERT INTO organization_groups(id, name, parent_id, position) VALUES(?, ?, ?, ?)", group.ID, group.Name, group.ParentID, index); errorValue != nil {
			_ = transaction.Rollback()
			return errorValue
		}
	}
	if errorValue := rewriteOrganizationProfileGroupReferences(ctx, transaction, groupAliases); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := writeOrganizationGroupsInitialized(ctx, transaction); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	return transaction.Commit()
}

func (service *Service) readOrganizationGroups(ctx context.Context) ([]orgGroupRecord, error) {
	groups, _, errorValue := service.readOrganizationGroupsWithInitialization(ctx)
	return groups, errorValue
}

func (service *Service) readOrganizationGroupsOrInitializeWithState(ctx context.Context) ([]orgGroupRecord, bool, error) {
	groups, isInitialized, errorValue := service.readOrganizationGroupsWithInitialization(ctx)
	if errorValue != nil {
		return nil, false, errorValue
	}
	if isInitialized {
		return groups, true, nil
	}
	if len(groups) > 0 {
		if errorValue := service.markOrganizationGroupsInitialized(ctx); errorValue != nil {
			return nil, false, errorValue
		}
		return groups, true, nil
	}
	if errorValue := service.markOrganizationGroupsInitialized(ctx); errorValue != nil {
		return nil, false, errorValue
	}
	return groups, true, nil
}

func (service *Service) readOrganizationGroupsWithInitialization(ctx context.Context) ([]orgGroupRecord, bool, error) {
	database, errorValue := service.openOrganizationDatabase(ctx)
	if errorValue != nil {
		return nil, false, errorValue
	}
	defer database.Close()
	isInitialized, errorValue := readOrganizationGroupsInitialized(ctx, database)
	if errorValue != nil {
		return nil, false, errorValue
	}
	rows, errorValue := database.QueryContext(ctx, "SELECT id, name, parent_id FROM organization_groups ORDER BY position, name")
	if errorValue != nil {
		return nil, false, errorValue
	}
	defer rows.Close()
	groups, errorValue := readOrganizationGroupsFromRows(rows)
	return groups, isInitialized, errorValue
}

type organizationGroupsQueryRunner interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

type organizationProfileGroupReferenceRewriter interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func readOrganizationGroupsFromQueryRunner(ctx context.Context, queryRunner organizationGroupsQueryRunner) ([]orgGroupRecord, error) {
	rows, errorValue := queryRunner.QueryContext(ctx, "SELECT id, name, parent_id FROM organization_groups ORDER BY position, name")
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	return readOrganizationGroupsFromRows(rows)
}

func readOrganizationGroupsFromRows(rows *sql.Rows) ([]orgGroupRecord, error) {
	groups := []orgGroupRecord{}
	for rows.Next() {
		var group orgGroupRecord
		if errorValue := rows.Scan(&group.ID, &group.Name, &group.ParentID); errorValue != nil {
			return nil, errorValue
		}
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

func rewriteOrganizationProfileGroupReferences(ctx context.Context, queryRunner organizationProfileGroupReferenceRewriter, groupAliases map[string]string) error {
	if len(groupAliases) == 0 {
		return nil
	}
	rows, errorValue := queryRunner.QueryContext(ctx, "SELECT profile_key, primary_group_id, group_ids FROM organization_profiles")
	if errorValue != nil {
		return errorValue
	}
	isRowsClosed := false
	defer func() {
		if !isRowsClosed {
			_ = rows.Close()
		}
	}()
	updates := []organizationProfileGroupReferenceUpdate{}
	for rows.Next() {
		var profileKey string
		var primaryGroupID string
		var groupIDsDocument string
		if errorValue := rows.Scan(&profileKey, &primaryGroupID, &groupIDsDocument); errorValue != nil {
			return errorValue
		}
		groupIDs := decodeOrganizationStringList(groupIDsDocument)
		nextPrimaryGroupID := canonicalOrganizationGroupID(primaryGroupID, groupAliases)
		nextGroupIDs := organizationGroupIDsWithPrimary(canonicalOrganizationGroupIDs(groupIDs, groupAliases), nextPrimaryGroupID)
		if primaryGroupID == nextPrimaryGroupID && organizationStringListsEqual(groupIDs, nextGroupIDs) {
			continue
		}
		nextGroupIDsDocument, errorValue := encodeOrganizationStringList(nextGroupIDs)
		if errorValue != nil {
			return errorValue
		}
		updates = append(updates, organizationProfileGroupReferenceUpdate{
			ProfileKey:     profileKey,
			PrimaryGroupID: nextPrimaryGroupID,
			GroupIDs:       nextGroupIDsDocument,
		})
	}
	if errorValue := rows.Err(); errorValue != nil {
		_ = rows.Close()
		isRowsClosed = true
		return errorValue
	}
	if errorValue := rows.Close(); errorValue != nil {
		return errorValue
	}
	isRowsClosed = true
	for _, update := range updates {
		if _, errorValue := queryRunner.ExecContext(ctx, "UPDATE organization_profiles SET primary_group_id = ?, group_ids = ? WHERE profile_key = ?", update.PrimaryGroupID, update.GroupIDs, update.ProfileKey); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

type organizationProfileGroupReferenceUpdate struct {
	ProfileKey     string
	PrimaryGroupID string
	GroupIDs       string
}

func canonicalOrganizationGroupIDs(groupIDs []string, groupAliases map[string]string) []string {
	result := []string{}
	for _, groupID := range groupIDs {
		result = append(result, canonicalOrganizationGroupID(groupID, groupAliases))
	}
	return normalizeOrganizationStringList(result)
}

func canonicalOrganizationGroupID(groupID string, groupAliases map[string]string) string {
	candidate := groupID
	seenGroupIDs := map[string]bool{}
	for {
		nextGroupID := groupAliases[candidate]
		if nextGroupID == "" || seenGroupIDs[candidate] {
			return candidate
		}
		seenGroupIDs[candidate] = true
		candidate = nextGroupID
	}
}

func organizationStringListsEqual(first []string, second []string) bool {
	if len(first) != len(second) {
		return false
	}
	for index := range first {
		if first[index] != second[index] {
			return false
		}
	}
	return true
}

func (service *Service) markOrganizationGroupsInitialized(ctx context.Context) error {
	database, errorValue := service.openOrganizationDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	return writeOrganizationGroupsInitialized(ctx, database)
}

type organizationGroupsInitializationWriter interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func writeOrganizationGroupsInitialized(ctx context.Context, writer organizationGroupsInitializationWriter) error {
	_, errorValue := writer.ExecContext(ctx, `
INSERT INTO organization_metadata(key, value) VALUES(?, ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		organizationGroupsInitializedKey,
		"true",
	)
	return errorValue
}

func readOrganizationGroupsInitialized(ctx context.Context, database *sql.DB) (bool, error) {
	var value string
	errorValue := database.QueryRowContext(ctx, "SELECT value FROM organization_metadata WHERE key = ?", organizationGroupsInitializedKey).Scan(&value)
	if errorValue == nil {
		return value == "true", nil
	}
	if errorValue == sql.ErrNoRows {
		return false, nil
	}
	return false, errorValue
}
