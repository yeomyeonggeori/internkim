package admind

import (
	"context"
	"database/sql"
)

const orgchartGroupsInitializedKey = "groups_initialized"

func (service *Service) writeOrgchartGroups(ctx context.Context, groups []orgGroupRecord) error {
	normalizedGroups, groupAliases := normalizeOrgchartGroupsWithAliases(groups)
	database, errorValue := service.openOrgchartDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	previousGroups, errorValue := readOrgchartGroupsFromQueryRunner(ctx, transaction)
	if errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	changedGroupIDs := changedOrgchartGroupIDs(previousGroups, normalizedGroups, groupAliases)
	if errorValue := invalidateOrgchartGroups(ctx, transaction, changedGroupIDs); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if _, errorValue := transaction.ExecContext(ctx, "DELETE FROM orgchart_groups"); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	for index, group := range normalizedGroups {
		if _, errorValue := transaction.ExecContext(ctx, "INSERT INTO orgchart_groups(id, name, position) VALUES(?, ?, ?)", group.ID, group.Name, index); errorValue != nil {
			_ = transaction.Rollback()
			return errorValue
		}
	}
	if errorValue := rewriteOrgchartProfileGroupReferences(ctx, transaction, groupAliases); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := writeOrgchartGroupsInitialized(ctx, transaction); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	return transaction.Commit()
}

func (service *Service) readOrgchartGroups(ctx context.Context) ([]orgGroupRecord, error) {
	groups, _, errorValue := service.readOrgchartGroupsWithInitialization(ctx)
	return groups, errorValue
}

func (service *Service) readOrgchartGroupsOrInitialize(ctx context.Context, fallbackGroups []orgGroupRecord) ([]orgGroupRecord, error) {
	groups, isInitialized, errorValue := service.readOrgchartGroupsWithInitialization(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	if isInitialized {
		return groups, nil
	}
	if len(groups) > 0 {
		return groups, service.markOrgchartGroupsInitialized(ctx)
	}
	if len(normalizeOrgchartGroups(fallbackGroups)) == 0 {
		return groups, nil
	}
	importedGroups, _, errorValue := service.importOrgchartGroupsIfUninitialized(ctx, fallbackGroups)
	return importedGroups, errorValue
}

func (service *Service) readOrgchartGroupsWithInitialization(ctx context.Context) ([]orgGroupRecord, bool, error) {
	database, errorValue := service.openOrgchartDatabase(ctx)
	if errorValue != nil {
		return nil, false, errorValue
	}
	defer database.Close()
	isInitialized, errorValue := readOrgchartGroupsInitialized(ctx, database)
	if errorValue != nil {
		return nil, false, errorValue
	}
	rows, errorValue := database.QueryContext(ctx, "SELECT id, name FROM orgchart_groups ORDER BY position, name")
	if errorValue != nil {
		return nil, false, errorValue
	}
	defer rows.Close()
	groups, errorValue := readOrgchartGroupsFromRows(rows)
	return groups, isInitialized, errorValue
}

func (service *Service) importOrgchartGroupsIfUninitialized(ctx context.Context, fallbackGroups []orgGroupRecord) ([]orgGroupRecord, bool, error) {
	normalizedFallbackGroups := normalizeOrgchartGroups(fallbackGroups)
	database, errorValue := service.openOrgchartDatabase(ctx)
	if errorValue != nil {
		return nil, false, errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return nil, false, errorValue
	}
	result, errorValue := transaction.ExecContext(ctx, "INSERT OR IGNORE INTO orgchart_metadata(key, value) VALUES(?, ?)", orgchartGroupsInitializedKey, "true")
	if errorValue != nil {
		_ = transaction.Rollback()
		return nil, false, errorValue
	}
	affectedRows, errorValue := result.RowsAffected()
	if errorValue != nil {
		_ = transaction.Rollback()
		return nil, false, errorValue
	}
	if affectedRows == 0 {
		groups, errorValue := readOrgchartGroupsFromQueryRunner(ctx, transaction)
		if errorValue != nil {
			_ = transaction.Rollback()
			return nil, false, errorValue
		}
		return groups, false, transaction.Commit()
	}
	for index, group := range normalizedFallbackGroups {
		if _, errorValue := transaction.ExecContext(ctx, `
INSERT INTO orgchart_groups(id, name, position) VALUES(?, ?, ?)
ON CONFLICT(id) DO UPDATE SET
	name = excluded.name,
	position = excluded.position`,
			group.ID,
			group.Name,
			index,
		); errorValue != nil {
			_ = transaction.Rollback()
			return nil, false, errorValue
		}
	}
	groups, errorValue := readOrgchartGroupsFromQueryRunner(ctx, transaction)
	if errorValue != nil {
		_ = transaction.Rollback()
		return nil, false, errorValue
	}
	return groups, true, transaction.Commit()
}

type orgchartGroupsQueryRunner interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

type orgchartProfileGroupReferenceRewriter interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func readOrgchartGroupsFromQueryRunner(ctx context.Context, queryRunner orgchartGroupsQueryRunner) ([]orgGroupRecord, error) {
	rows, errorValue := queryRunner.QueryContext(ctx, "SELECT id, name FROM orgchart_groups ORDER BY position, name")
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	return readOrgchartGroupsFromRows(rows)
}

func readOrgchartGroupsFromRows(rows *sql.Rows) ([]orgGroupRecord, error) {
	groups := []orgGroupRecord{}
	for rows.Next() {
		var group orgGroupRecord
		if errorValue := rows.Scan(&group.ID, &group.Name); errorValue != nil {
			return nil, errorValue
		}
		groups = append(groups, group)
	}
	return groups, rows.Err()
}

func rewriteOrgchartProfileGroupReferences(ctx context.Context, queryRunner orgchartProfileGroupReferenceRewriter, groupAliases map[string]string) error {
	if len(groupAliases) == 0 {
		return nil
	}
	rows, errorValue := queryRunner.QueryContext(ctx, "SELECT profile_key, primary_group_id, group_ids FROM orgchart_profiles")
	if errorValue != nil {
		return errorValue
	}
	isRowsClosed := false
	defer func() {
		if !isRowsClosed {
			_ = rows.Close()
		}
	}()
	updates := []orgchartProfileGroupReferenceUpdate{}
	for rows.Next() {
		var profileKey string
		var primaryGroupID string
		var groupIDsDocument string
		if errorValue := rows.Scan(&profileKey, &primaryGroupID, &groupIDsDocument); errorValue != nil {
			return errorValue
		}
		groupIDs := decodeOrgchartStringList(groupIDsDocument)
		nextPrimaryGroupID := canonicalOrgchartGroupID(primaryGroupID, groupAliases)
		nextGroupIDs := orgchartGroupIDsWithPrimary(canonicalOrgchartGroupIDs(groupIDs, groupAliases), nextPrimaryGroupID)
		if primaryGroupID == nextPrimaryGroupID && orgchartStringListsEqual(groupIDs, nextGroupIDs) {
			continue
		}
		nextGroupIDsDocument, errorValue := encodeOrgchartStringList(nextGroupIDs)
		if errorValue != nil {
			return errorValue
		}
		updates = append(updates, orgchartProfileGroupReferenceUpdate{
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
		if _, errorValue := queryRunner.ExecContext(ctx, "UPDATE orgchart_profiles SET primary_group_id = ?, group_ids = ? WHERE profile_key = ?", update.PrimaryGroupID, update.GroupIDs, update.ProfileKey); errorValue != nil {
			return errorValue
		}
	}
	return nil
}

type orgchartProfileGroupReferenceUpdate struct {
	ProfileKey     string
	PrimaryGroupID string
	GroupIDs       string
}

func canonicalOrgchartGroupIDs(groupIDs []string, groupAliases map[string]string) []string {
	result := []string{}
	for _, groupID := range groupIDs {
		result = append(result, canonicalOrgchartGroupID(groupID, groupAliases))
	}
	return normalizeOrgchartStringList(result)
}

func canonicalOrgchartGroupID(groupID string, groupAliases map[string]string) string {
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

func orgchartStringListsEqual(first []string, second []string) bool {
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

func (service *Service) markOrgchartGroupsInitialized(ctx context.Context) error {
	database, errorValue := service.openOrgchartDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	return writeOrgchartGroupsInitialized(ctx, database)
}

type orgchartGroupsInitializationWriter interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func writeOrgchartGroupsInitialized(ctx context.Context, writer orgchartGroupsInitializationWriter) error {
	_, errorValue := writer.ExecContext(ctx, `
INSERT INTO orgchart_metadata(key, value) VALUES(?, ?)
ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
		orgchartGroupsInitializedKey,
		"true",
	)
	return errorValue
}

func readOrgchartGroupsInitialized(ctx context.Context, database *sql.DB) (bool, error) {
	var value string
	errorValue := database.QueryRowContext(ctx, "SELECT value FROM orgchart_metadata WHERE key = ?", orgchartGroupsInitializedKey).Scan(&value)
	if errorValue == nil {
		return value == "true", nil
	}
	if errorValue == sql.ErrNoRows {
		return false, nil
	}
	return false, errorValue
}
