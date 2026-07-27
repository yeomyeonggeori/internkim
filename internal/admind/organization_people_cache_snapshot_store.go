package admind

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

func (service *Service) readOrganizationPeopleCacheSnapshots(ctx context.Context, keys []organizationPeopleCacheKey) (map[organizationPeopleCacheKey]organizationPeopleCacheSnapshot, error) {
	uniqueKeys := uniqueOrganizationPeopleCacheKeys(keys)
	result := make(map[organizationPeopleCacheKey]organizationPeopleCacheSnapshot, len(uniqueKeys))
	if len(uniqueKeys) == 0 {
		return result, nil
	}
	database, errorValue := service.openOrganizationDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	query, arguments := organizationPeopleCacheSnapshotQuery(uniqueKeys)
	rows, errorValue := database.QueryContext(ctx, query, arguments...)
	if errorValue != nil {
		return nil, fmt.Errorf("read organization people cache snapshots: %w", errorValue)
	}
	defer rows.Close()
	for rows.Next() {
		var key organizationPeopleCacheKey
		var snapshot organizationPeopleCacheSnapshot
		var isDirty int
		var sourceRevision sql.NullString
		var schemaVersion sql.NullInt64
		var payloadJSON []byte
		if errorValue := rows.Scan(&key.Kind, &key.Key, &snapshot.Revision, &isDirty, &snapshot.ActiveMutations, &sourceRevision, &schemaVersion, &payloadJSON); errorValue != nil {
			return nil, fmt.Errorf("scan organization people cache snapshot: %w", errorValue)
		}
		snapshot.IsDirty = isDirty == 1
		snapshot.Found = schemaVersion.Valid
		if sourceRevision.Valid {
			snapshot.SourceRevision = sourceRevision.String
		}
		if schemaVersion.Valid {
			snapshot.SchemaVersion = int(schemaVersion.Int64)
			snapshot.PayloadJSON = payloadJSON
		}
		result[key] = snapshot
	}
	if errorValue := rows.Err(); errorValue != nil {
		return nil, fmt.Errorf("iterate organization people cache snapshots: %w", errorValue)
	}
	return result, nil
}

func organizationPeopleCacheSnapshotQuery(keys []organizationPeopleCacheKey) (string, []any) {
	valueRows := make([]string, 0, len(keys))
	arguments := make([]any, 0, len(keys)*2)
	for _, key := range keys {
		valueRows = append(valueRows, "(?, ?)")
		arguments = append(arguments, string(key.Kind), key.Key)
	}
	query := `
	WITH requested(cache_kind, cache_key) AS (VALUES ` + strings.Join(valueRows, ",") + `)
	SELECT
		requested.cache_kind,
		requested.cache_key,
		COALESCE(states.revision, 0),
		COALESCE(states.is_dirty, 0),
		COALESCE(states.active_mutations, 0),
		entries.source_revision,
		entries.schema_version,
		entries.payload_json
	FROM requested
	LEFT JOIN organization_people_cache_states AS states
		USING(cache_kind, cache_key)
	LEFT JOIN organization_people_cache_entries AS entries
		USING(cache_kind, cache_key)`
	return query, arguments
}
