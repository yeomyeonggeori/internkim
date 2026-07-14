package admind

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type orgchartPeopleCacheKind string

const (
	orgchartPeopleCacheList   orgchartPeopleCacheKind = "list"
	orgchartPeopleCachePerson orgchartPeopleCacheKind = "person"
	orgchartPeopleCacheGroups orgchartPeopleCacheKind = "groups"
)

type orgchartPeopleCacheKey struct {
	Kind orgchartPeopleCacheKind
	Key  string
}

type orgchartPeopleCacheSnapshot struct {
	Revision       int64
	IsDirty        bool
	SourceRevision string
	SchemaVersion  int
	PayloadJSON    []byte
	Found          bool
}

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
		if errorValue := rows.Scan(&key.Kind, &key.Key, &snapshot.Revision, &isDirty, &sourceRevision, &schemaVersion, &payloadJSON); errorValue != nil {
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

func (service *Service) writeOrgchartPeopleCachePayloadIfCurrent(ctx context.Context, key orgchartPeopleCacheKey, revision int64, sourceRevision string, payloadJSON []byte) (bool, error) {
	database, errorValue := service.openOrgchartDatabase(ctx)
	if errorValue != nil {
		return false, errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return false, fmt.Errorf("begin orgchart people cache write: %w", errorValue)
	}
	defer transaction.Rollback()
	if errorValue := ensureOrgchartPeopleCacheState(ctx, transaction, key); errorValue != nil {
		return false, errorValue
	}
	result, errorValue := transaction.ExecContext(ctx, `
		INSERT INTO orgchart_people_cache_entries(
			cache_kind, cache_key, source_revision, schema_version, payload_json, cached_at
		)
		SELECT cache_kind, cache_key, ?, ?, ?, ?
		FROM orgchart_people_cache_states
		WHERE cache_kind = ? AND cache_key = ? AND revision = ? AND is_dirty = 0
		ON CONFLICT(cache_kind, cache_key) DO UPDATE SET
			source_revision = excluded.source_revision,
			schema_version = excluded.schema_version,
			payload_json = excluded.payload_json,
			cached_at = excluded.cached_at`,
		sourceRevision,
		orgchartPeopleCacheSchemaVersion,
		payloadJSON,
		time.Now().UTC().Format(time.RFC3339Nano),
		string(key.Kind),
		key.Key,
		revision,
	)
	if errorValue != nil {
		return false, fmt.Errorf("write orgchart people cache payload: %w", errorValue)
	}
	rowsAffected, errorValue := result.RowsAffected()
	if errorValue != nil {
		return false, fmt.Errorf("read orgchart people cache write result: %w", errorValue)
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return false, fmt.Errorf("commit orgchart people cache write: %w", errorValue)
	}
	return rowsAffected == 1, nil
}

func (service *Service) beginOrgchartPeopleCacheMutation(ctx context.Context, keys []orgchartPeopleCacheKey) error {
	return service.updateOrgchartPeopleCacheMutation(ctx, keys, true)
}

func (service *Service) completeOrgchartPeopleCacheMutation(ctx context.Context, keys []orgchartPeopleCacheKey) error {
	return service.updateOrgchartPeopleCacheMutation(ctx, keys, false)
}

func (service *Service) updateOrgchartPeopleCacheMutation(ctx context.Context, keys []orgchartPeopleCacheKey, isBeginning bool) error {
	uniqueKeys := uniqueOrgchartPeopleCacheKeys(keys)
	if len(uniqueKeys) == 0 {
		return nil
	}
	database, errorValue := service.openOrgchartDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return fmt.Errorf("begin orgchart people cache mutation: %w", errorValue)
	}
	defer transaction.Rollback()
	for _, key := range uniqueKeys {
		if errorValue := ensureOrgchartPeopleCacheState(ctx, transaction, key); errorValue != nil {
			return errorValue
		}
		if isBeginning {
			if _, errorValue := transaction.ExecContext(ctx, `UPDATE orgchart_people_cache_states SET revision = revision + 1, is_dirty = 1 WHERE cache_kind = ? AND cache_key = ?`, string(key.Kind), key.Key); errorValue != nil {
				return fmt.Errorf("mark orgchart people cache dirty: %w", errorValue)
			}
			if _, errorValue := transaction.ExecContext(ctx, `DELETE FROM orgchart_people_cache_entries WHERE cache_kind = ? AND cache_key = ?`, string(key.Kind), key.Key); errorValue != nil {
				return fmt.Errorf("delete orgchart people cache entry: %w", errorValue)
			}
			continue
		}
		if _, errorValue := transaction.ExecContext(ctx, `UPDATE orgchart_people_cache_states SET is_dirty = 0 WHERE cache_kind = ? AND cache_key = ?`, string(key.Kind), key.Key); errorValue != nil {
			return fmt.Errorf("clear orgchart people cache dirty state: %w", errorValue)
		}
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return fmt.Errorf("commit orgchart people cache mutation: %w", errorValue)
	}
	return nil
}

func ensureOrgchartPeopleCacheState(ctx context.Context, transaction *sql.Tx, key orgchartPeopleCacheKey) error {
	_, errorValue := transaction.ExecContext(ctx, `INSERT INTO orgchart_people_cache_states(cache_kind, cache_key) VALUES(?, ?) ON CONFLICT(cache_kind, cache_key) DO NOTHING`, string(key.Kind), key.Key)
	if errorValue != nil {
		return fmt.Errorf("ensure orgchart people cache state: %w", errorValue)
	}
	return nil
}

func uniqueOrgchartPeopleCacheKeys(keys []orgchartPeopleCacheKey) []orgchartPeopleCacheKey {
	seen := make(map[orgchartPeopleCacheKey]struct{}, len(keys))
	result := make([]orgchartPeopleCacheKey, 0, len(keys))
	for _, key := range keys {
		if key.Kind == "" || key.Key == "" {
			continue
		}
		if _, found := seen[key]; found {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, key)
	}
	return result
}
