package admind

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type organizationPeopleCacheWrite struct {
	Key                     organizationPeopleCacheKey
	Revision                int64
	SourceRevision          string
	HasExpectedListRevision bool
	ExpectedListRevision    int64
	PayloadJSON             []byte
}

func (service *Service) writeOrganizationPeopleCachePayloadIfCurrent(ctx context.Context, key organizationPeopleCacheKey, revision int64, sourceRevision string, payloadJSON []byte) (bool, error) {
	writtenByKey, errorValue := service.writeOrganizationPeopleCachePayloadsIfCurrent(ctx, []organizationPeopleCacheWrite{{
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

func (service *Service) writeOrganizationPeopleCachePayloadsIfCurrent(ctx context.Context, writes []organizationPeopleCacheWrite) (map[organizationPeopleCacheKey]bool, error) {
	writtenByKey := make(map[organizationPeopleCacheKey]bool, len(writes))
	if len(writes) == 0 {
		return writtenByKey, nil
	}
	database, errorValue := service.openOrganizationDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return nil, fmt.Errorf("begin organization people cache writes: %w", errorValue)
	}
	defer transaction.Rollback()
	cachedAt := time.Now().UTC().Format(time.RFC3339Nano)
	currentListRevisions := map[int64]bool{}
	for _, write := range writes {
		if write.HasExpectedListRevision {
			isCurrent, found := currentListRevisions[write.ExpectedListRevision]
			if !found {
				isCurrent, errorValue = isExpectedOrganizationPeopleListRevisionForWrite(ctx, transaction, write.ExpectedListRevision)
				if errorValue != nil {
					return nil, errorValue
				}
				currentListRevisions[write.ExpectedListRevision] = isCurrent
			}
			if !isCurrent {
				writtenByKey[write.Key] = false
				continue
			}
		}
		written, errorValue := writeOrganizationPeopleCachePayloadIfCurrentTransaction(ctx, transaction, write, cachedAt)
		if errorValue != nil {
			return nil, errorValue
		}
		writtenByKey[write.Key] = written
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return nil, fmt.Errorf("commit organization people cache writes: %w", errorValue)
	}
	return writtenByKey, nil
}

func writeOrganizationPeopleCachePayloadIfCurrentTransaction(ctx context.Context, transaction *sql.Tx, write organizationPeopleCacheWrite, cachedAt string) (bool, error) {
	if errorValue := ensureOrganizationPeopleCacheState(ctx, transaction, write.Key); errorValue != nil {
		return false, errorValue
	}
	result, errorValue := transaction.ExecContext(ctx, `
		INSERT INTO organization_people_cache_entries(
			cache_kind, cache_key, source_revision, schema_version, payload_json, cached_at
		)
		SELECT cache_kind, cache_key, ?, ?, ?, ?
		FROM organization_people_cache_states
		WHERE cache_kind = ? AND cache_key = ? AND revision = ? AND is_dirty = 0 AND active_mutations = 0
		ON CONFLICT(cache_kind, cache_key) DO UPDATE SET
			source_revision = excluded.source_revision,
			schema_version = excluded.schema_version,
			payload_json = excluded.payload_json,
			cached_at = excluded.cached_at`,
		write.SourceRevision,
		organizationPeopleCacheSchemaVersion,
		write.PayloadJSON,
		cachedAt,
		string(write.Key.Kind),
		write.Key.Key,
		write.Revision,
	)
	if errorValue != nil {
		return false, fmt.Errorf("write organization people cache payload for %s/%s: %w", write.Key.Kind, write.Key.Key, errorValue)
	}
	rowsAffected, errorValue := result.RowsAffected()
	if errorValue != nil {
		return false, fmt.Errorf("read organization people cache write result: %w", errorValue)
	}
	return rowsAffected == 1, nil
}

func isExpectedOrganizationPeopleListRevisionForWrite(ctx context.Context, transaction *sql.Tx, expectedRevision int64) (bool, error) {
	listKey := organizationPeopleCacheKey{Kind: organizationPeopleCacheList, Key: organizationPeopleCacheSingletonKey}
	if errorValue := ensureOrganizationPeopleCacheState(ctx, transaction, listKey); errorValue != nil {
		return false, errorValue
	}
	var isCurrent bool
	errorValue := transaction.QueryRowContext(ctx, `SELECT EXISTS(
		SELECT 1 FROM organization_people_cache_states
		WHERE cache_kind = ? AND cache_key = ? AND revision = ? AND is_dirty = 0 AND active_mutations = 0
	)`, string(listKey.Kind), listKey.Key, expectedRevision).Scan(&isCurrent)
	if errorValue != nil {
		return false, fmt.Errorf("validate organization people list revision for cache write: %w", errorValue)
	}
	return isCurrent, nil
}

func (service *Service) deleteOrganizationPeopleCacheEntries(ctx context.Context, keys []organizationPeopleCacheKey) error {
	uniqueKeys := uniqueOrganizationPeopleCacheKeys(keys)
	if len(uniqueKeys) == 0 {
		return nil
	}
	database, errorValue := service.openOrganizationDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return fmt.Errorf("begin organization people cache delete: %w", errorValue)
	}
	defer transaction.Rollback()
	for _, key := range uniqueKeys {
		if _, errorValue := transaction.ExecContext(ctx, `DELETE FROM organization_people_cache_entries WHERE cache_kind = ? AND cache_key = ?`, string(key.Kind), key.Key); errorValue != nil {
			return fmt.Errorf("delete organization people cache entry: %w", errorValue)
		}
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return fmt.Errorf("commit organization people cache delete: %w", errorValue)
	}
	return nil
}
