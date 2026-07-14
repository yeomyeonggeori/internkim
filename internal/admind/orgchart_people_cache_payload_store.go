package admind

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type orgchartPeopleCacheWrite struct {
	Key            orgchartPeopleCacheKey
	Revision       int64
	SourceRevision string
	PayloadJSON    []byte
}

func (service *Service) writeOrgchartPeopleCachePayloadIfCurrent(ctx context.Context, key orgchartPeopleCacheKey, revision int64, sourceRevision string, payloadJSON []byte) (bool, error) {
	writtenByKey, errorValue := service.writeOrgchartPeopleCachePayloadsIfCurrent(ctx, []orgchartPeopleCacheWrite{{
		Key:            key,
		Revision:       revision,
		SourceRevision: sourceRevision,
		PayloadJSON:    payloadJSON,
	}})
	if errorValue != nil {
		return false, errorValue
	}
	return writtenByKey[key], nil
}

func (service *Service) writeOrgchartPeopleCachePayloadsIfCurrent(ctx context.Context, writes []orgchartPeopleCacheWrite) (map[orgchartPeopleCacheKey]bool, error) {
	writtenByKey := make(map[orgchartPeopleCacheKey]bool, len(writes))
	if len(writes) == 0 {
		return writtenByKey, nil
	}
	database, errorValue := service.openOrgchartDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return nil, fmt.Errorf("begin orgchart people cache writes: %w", errorValue)
	}
	defer transaction.Rollback()
	cachedAt := time.Now().UTC().Format(time.RFC3339Nano)
	for _, write := range writes {
		written, errorValue := writeOrgchartPeopleCachePayloadIfCurrentTransaction(ctx, transaction, write, cachedAt)
		if errorValue != nil {
			return nil, errorValue
		}
		writtenByKey[write.Key] = written
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return nil, fmt.Errorf("commit orgchart people cache writes: %w", errorValue)
	}
	return writtenByKey, nil
}

func writeOrgchartPeopleCachePayloadIfCurrentTransaction(ctx context.Context, transaction *sql.Tx, write orgchartPeopleCacheWrite, cachedAt string) (bool, error) {
	if errorValue := ensureOrgchartPeopleCacheState(ctx, transaction, write.Key); errorValue != nil {
		return false, errorValue
	}
	result, errorValue := transaction.ExecContext(ctx, `
		INSERT INTO orgchart_people_cache_entries(
			cache_kind, cache_key, source_revision, schema_version, payload_json, cached_at
		)
		SELECT cache_kind, cache_key, ?, ?, ?, ?
		FROM orgchart_people_cache_states
		WHERE cache_kind = ? AND cache_key = ? AND revision = ? AND is_dirty = 0 AND active_mutations = 0
		ON CONFLICT(cache_kind, cache_key) DO UPDATE SET
			source_revision = excluded.source_revision,
			schema_version = excluded.schema_version,
			payload_json = excluded.payload_json,
			cached_at = excluded.cached_at`,
		write.SourceRevision,
		orgchartPeopleCacheSchemaVersion,
		write.PayloadJSON,
		cachedAt,
		string(write.Key.Kind),
		write.Key.Key,
		write.Revision,
	)
	if errorValue != nil {
		return false, fmt.Errorf("write orgchart people cache payload for %s/%s: %w", write.Key.Kind, write.Key.Key, errorValue)
	}
	rowsAffected, errorValue := result.RowsAffected()
	if errorValue != nil {
		return false, fmt.Errorf("read orgchart people cache write result: %w", errorValue)
	}
	return rowsAffected == 1, nil
}

func (service *Service) deleteOrgchartPeopleCacheEntries(ctx context.Context, keys []orgchartPeopleCacheKey) error {
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
		return fmt.Errorf("begin orgchart people cache delete: %w", errorValue)
	}
	defer transaction.Rollback()
	for _, key := range uniqueKeys {
		if _, errorValue := transaction.ExecContext(ctx, `DELETE FROM orgchart_people_cache_entries WHERE cache_kind = ? AND cache_key = ?`, string(key.Kind), key.Key); errorValue != nil {
			return fmt.Errorf("delete orgchart people cache entry: %w", errorValue)
		}
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return fmt.Errorf("commit orgchart people cache delete: %w", errorValue)
	}
	return nil
}
