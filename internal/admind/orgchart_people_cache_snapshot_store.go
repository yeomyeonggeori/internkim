package admind

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

func (service *Service) readOrgchartPeopleCacheSnapshots(ctx context.Context, keys []orgchartPeopleCacheKey) (map[orgchartPeopleCacheKey]orgchartPeopleCacheSnapshot, error) {
	uniqueKeys := uniqueOrgchartPeopleCacheKeys(keys)
	result := make(map[orgchartPeopleCacheKey]orgchartPeopleCacheSnapshot, len(uniqueKeys))
	if len(uniqueKeys) == 0 {
		return result, nil
	}
	database, errorValue := service.openOrgchartDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	query, arguments := orgchartPeopleCacheSnapshotQuery(uniqueKeys)
	rows, errorValue := database.QueryContext(ctx, query, arguments...)
	if errorValue != nil {
		return nil, fmt.Errorf("read orgchart people cache snapshots: %w", errorValue)
	}
	defer rows.Close()
	for rows.Next() {
		var key orgchartPeopleCacheKey
		var snapshot orgchartPeopleCacheSnapshot
		var isDirty int
		var sourceRevision sql.NullString
		var schemaVersion sql.NullInt64
		var payloadJSON []byte
		if errorValue := rows.Scan(&key.Kind, &key.Key, &snapshot.Revision, &isDirty, &snapshot.ActiveMutations, &sourceRevision, &schemaVersion, &payloadJSON); errorValue != nil {
			return nil, fmt.Errorf("scan orgchart people cache snapshot: %w", errorValue)
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
		return nil, fmt.Errorf("iterate orgchart people cache snapshots: %w", errorValue)
	}
	return result, nil
}

func orgchartPeopleCacheSnapshotQuery(keys []orgchartPeopleCacheKey) (string, []any) {
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
	LEFT JOIN orgchart_people_cache_states AS states
		USING(cache_kind, cache_key)
	LEFT JOIN orgchart_people_cache_entries AS entries
		USING(cache_kind, cache_key)`
	return query, arguments
}
