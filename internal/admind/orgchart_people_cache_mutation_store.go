package admind

import (
	"context"
	"database/sql"
	"fmt"
)

func (service *Service) beginOrgchartPeopleCacheMutation(ctx context.Context, keys []orgchartPeopleCacheKey) error {
	return service.updateOrgchartPeopleCacheMutation(ctx, keys, true)
}

func (service *Service) beginOrgchartUserCacheMutation(ctx context.Context, keys []orgchartPeopleCacheKey) ([]orgchartPeopleCacheKey, error) {
	uniqueKeys := uniqueOrgchartPeopleCacheKeys(keys)
	if len(uniqueKeys) == 0 {
		return nil, nil
	}
	database, errorValue := service.openOrgchartDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return nil, fmt.Errorf("begin orgchart user cache mutation: %w", errorValue)
	}
	defer transaction.Rollback()
	activeKeys, errorValue := beginOrgchartUserCacheMutations(ctx, transaction, uniqueKeys)
	if errorValue != nil {
		return nil, errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return nil, fmt.Errorf("commit orgchart user cache mutation: %w", errorValue)
	}
	return activeKeys, nil
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
	if isBeginning {
		if errorValue := beginOrgchartPeopleCacheMutations(ctx, transaction, uniqueKeys); errorValue != nil {
			return errorValue
		}
	} else if errorValue := completeOrgchartPeopleCacheMutations(ctx, transaction, uniqueKeys); errorValue != nil {
		return errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return fmt.Errorf("commit orgchart people cache mutation: %w", errorValue)
	}
	return nil
}

func beginOrgchartPeopleCacheMutations(ctx context.Context, transaction *sql.Tx, keys []orgchartPeopleCacheKey) error {
	for _, key := range uniqueOrgchartPeopleCacheKeys(keys) {
		if errorValue := ensureOrgchartPeopleCacheState(ctx, transaction, key); errorValue != nil {
			return errorValue
		}
		if _, errorValue := transaction.ExecContext(ctx, `UPDATE orgchart_people_cache_states SET revision = revision + 1, is_dirty = 1, active_mutations = active_mutations + 1 WHERE cache_kind = ? AND cache_key = ?`, string(key.Kind), key.Key); errorValue != nil {
			return fmt.Errorf("begin orgchart people cache mutation: %w", errorValue)
		}
		if _, errorValue := transaction.ExecContext(ctx, `DELETE FROM orgchart_people_cache_entries WHERE cache_kind = ? AND cache_key = ?`, string(key.Kind), key.Key); errorValue != nil {
			return fmt.Errorf("delete orgchart people cache entry: %w", errorValue)
		}
	}
	return nil
}

func beginOrgchartUserCacheMutations(ctx context.Context, transaction *sql.Tx, keys []orgchartPeopleCacheKey) ([]orgchartPeopleCacheKey, error) {
	activeKeys := make([]orgchartPeopleCacheKey, 0, len(keys))
	for _, key := range keys {
		if key.Kind == orgchartPeopleCacheList {
			if errorValue := ensureOrgchartPeopleCacheState(ctx, transaction, key); errorValue != nil {
				return nil, errorValue
			}
		}
		result, errorValue := transaction.ExecContext(ctx, `UPDATE orgchart_people_cache_states SET revision = revision + 1, is_dirty = 1, active_mutations = active_mutations + 1 WHERE cache_kind = ? AND cache_key = ?`, string(key.Kind), key.Key)
		if errorValue != nil {
			return nil, fmt.Errorf("begin orgchart user cache mutation: %w", errorValue)
		}
		rowsAffected, errorValue := result.RowsAffected()
		if errorValue != nil {
			return nil, fmt.Errorf("read orgchart user cache mutation result: %w", errorValue)
		}
		if rowsAffected == 0 {
			continue
		}
		if _, errorValue := transaction.ExecContext(ctx, `DELETE FROM orgchart_people_cache_entries WHERE cache_kind = ? AND cache_key = ?`, string(key.Kind), key.Key); errorValue != nil {
			return nil, fmt.Errorf("delete orgchart people cache entry: %w", errorValue)
		}
		activeKeys = append(activeKeys, key)
	}
	return activeKeys, nil
}

func completeOrgchartPeopleCacheMutations(ctx context.Context, transaction *sql.Tx, keys []orgchartPeopleCacheKey) error {
	for _, key := range uniqueOrgchartPeopleCacheKeys(keys) {
		if errorValue := ensureOrgchartPeopleCacheState(ctx, transaction, key); errorValue != nil {
			return errorValue
		}
		if _, errorValue := transaction.ExecContext(ctx, `UPDATE orgchart_people_cache_states SET active_mutations = CASE WHEN active_mutations > 0 THEN active_mutations - 1 ELSE 0 END, is_dirty = CASE WHEN active_mutations <= 1 THEN 0 ELSE 1 END WHERE cache_kind = ? AND cache_key = ?`, string(key.Kind), key.Key); errorValue != nil {
			return fmt.Errorf("complete orgchart people cache mutation: %w", errorValue)
		}
	}
	return nil
}

func invalidateOrgchartPeopleCacheKeys(ctx context.Context, transaction *sql.Tx, keys []orgchartPeopleCacheKey, isDirty bool) error {
	for _, key := range uniqueOrgchartPeopleCacheKeys(keys) {
		if errorValue := ensureOrgchartPeopleCacheState(ctx, transaction, key); errorValue != nil {
			return errorValue
		}
		if _, errorValue := transaction.ExecContext(ctx, `UPDATE orgchart_people_cache_states SET revision = revision + 1, is_dirty = CASE WHEN ? = 1 OR active_mutations > 0 THEN 1 ELSE 0 END WHERE cache_kind = ? AND cache_key = ?`, isDirty, string(key.Kind), key.Key); errorValue != nil {
			return fmt.Errorf("invalidate orgchart people cache state: %w", errorValue)
		}
		if _, errorValue := transaction.ExecContext(ctx, `DELETE FROM orgchart_people_cache_entries WHERE cache_kind = ? AND cache_key = ?`, string(key.Kind), key.Key); errorValue != nil {
			return fmt.Errorf("delete orgchart people cache entry: %w", errorValue)
		}
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
