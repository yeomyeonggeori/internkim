package admind

import (
	"context"
	"database/sql"
	"fmt"
)

func (service *Service) beginOrganizationPeopleCacheMutation(ctx context.Context, keys []organizationPeopleCacheKey) error {
	return service.updateOrganizationPeopleCacheMutation(ctx, keys, true)
}

func (service *Service) beginOrganizationUserCacheMutation(ctx context.Context, keys []organizationPeopleCacheKey) ([]organizationPeopleCacheKey, error) {
	uniqueKeys := uniqueOrganizationPeopleCacheKeys(keys)
	if len(uniqueKeys) == 0 {
		return nil, nil
	}
	database, errorValue := service.openOrganizationDatabase(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return nil, fmt.Errorf("begin organization user cache mutation: %w", errorValue)
	}
	defer transaction.Rollback()
	activeKeys, errorValue := beginOrganizationUserCacheMutations(ctx, transaction, uniqueKeys)
	if errorValue != nil {
		return nil, errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return nil, fmt.Errorf("commit organization user cache mutation: %w", errorValue)
	}
	return activeKeys, nil
}

func (service *Service) completeOrganizationPeopleCacheMutation(ctx context.Context, keys []organizationPeopleCacheKey) error {
	return service.updateOrganizationPeopleCacheMutation(ctx, keys, false)
}

func (service *Service) updateOrganizationPeopleCacheMutation(ctx context.Context, keys []organizationPeopleCacheKey, isBeginning bool) error {
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
		return fmt.Errorf("begin organization people cache mutation: %w", errorValue)
	}
	defer transaction.Rollback()
	if isBeginning {
		if errorValue := beginOrganizationPeopleCacheMutations(ctx, transaction, uniqueKeys); errorValue != nil {
			return errorValue
		}
	} else if errorValue := completeOrganizationPeopleCacheMutations(ctx, transaction, uniqueKeys); errorValue != nil {
		return errorValue
	}
	if errorValue := transaction.Commit(); errorValue != nil {
		return fmt.Errorf("commit organization people cache mutation: %w", errorValue)
	}
	return nil
}

func beginOrganizationPeopleCacheMutations(ctx context.Context, transaction *sql.Tx, keys []organizationPeopleCacheKey) error {
	for _, key := range uniqueOrganizationPeopleCacheKeys(keys) {
		if errorValue := ensureOrganizationPeopleCacheState(ctx, transaction, key); errorValue != nil {
			return errorValue
		}
		if _, errorValue := transaction.ExecContext(ctx, `UPDATE organization_people_cache_states SET revision = revision + 1, is_dirty = 1, active_mutations = active_mutations + 1 WHERE cache_kind = ? AND cache_key = ?`, string(key.Kind), key.Key); errorValue != nil {
			return fmt.Errorf("begin organization people cache mutation: %w", errorValue)
		}
		if _, errorValue := transaction.ExecContext(ctx, `DELETE FROM organization_people_cache_entries WHERE cache_kind = ? AND cache_key = ?`, string(key.Kind), key.Key); errorValue != nil {
			return fmt.Errorf("delete organization people cache entry: %w", errorValue)
		}
	}
	return nil
}

func beginOrganizationUserCacheMutations(ctx context.Context, transaction *sql.Tx, keys []organizationPeopleCacheKey) ([]organizationPeopleCacheKey, error) {
	activeKeys := make([]organizationPeopleCacheKey, 0, len(keys))
	for _, key := range keys {
		if key.Kind == organizationPeopleCacheList {
			if errorValue := ensureOrganizationPeopleCacheState(ctx, transaction, key); errorValue != nil {
				return nil, errorValue
			}
		}
		result, errorValue := transaction.ExecContext(ctx, `UPDATE organization_people_cache_states SET revision = revision + 1, is_dirty = 1, active_mutations = active_mutations + 1 WHERE cache_kind = ? AND cache_key = ?`, string(key.Kind), key.Key)
		if errorValue != nil {
			return nil, fmt.Errorf("begin organization user cache mutation: %w", errorValue)
		}
		rowsAffected, errorValue := result.RowsAffected()
		if errorValue != nil {
			return nil, fmt.Errorf("read organization user cache mutation result: %w", errorValue)
		}
		if rowsAffected == 0 {
			continue
		}
		if _, errorValue := transaction.ExecContext(ctx, `DELETE FROM organization_people_cache_entries WHERE cache_kind = ? AND cache_key = ?`, string(key.Kind), key.Key); errorValue != nil {
			return nil, fmt.Errorf("delete organization people cache entry: %w", errorValue)
		}
		activeKeys = append(activeKeys, key)
	}
	return activeKeys, nil
}

func completeOrganizationPeopleCacheMutations(ctx context.Context, transaction *sql.Tx, keys []organizationPeopleCacheKey) error {
	for _, key := range uniqueOrganizationPeopleCacheKeys(keys) {
		if errorValue := ensureOrganizationPeopleCacheState(ctx, transaction, key); errorValue != nil {
			return errorValue
		}
		if _, errorValue := transaction.ExecContext(ctx, `UPDATE organization_people_cache_states SET active_mutations = CASE WHEN active_mutations > 0 THEN active_mutations - 1 ELSE 0 END, is_dirty = CASE WHEN active_mutations <= 1 THEN 0 ELSE 1 END WHERE cache_kind = ? AND cache_key = ?`, string(key.Kind), key.Key); errorValue != nil {
			return fmt.Errorf("complete organization people cache mutation: %w", errorValue)
		}
	}
	return nil
}

func invalidateOrganizationPeopleCacheKeys(ctx context.Context, transaction *sql.Tx, keys []organizationPeopleCacheKey, isDirty bool) error {
	for _, key := range uniqueOrganizationPeopleCacheKeys(keys) {
		if errorValue := ensureOrganizationPeopleCacheState(ctx, transaction, key); errorValue != nil {
			return errorValue
		}
		if _, errorValue := transaction.ExecContext(ctx, `UPDATE organization_people_cache_states SET revision = revision + 1, is_dirty = CASE WHEN ? = 1 OR active_mutations > 0 THEN 1 ELSE 0 END WHERE cache_kind = ? AND cache_key = ?`, isDirty, string(key.Kind), key.Key); errorValue != nil {
			return fmt.Errorf("invalidate organization people cache state: %w", errorValue)
		}
		if _, errorValue := transaction.ExecContext(ctx, `DELETE FROM organization_people_cache_entries WHERE cache_kind = ? AND cache_key = ?`, string(key.Kind), key.Key); errorValue != nil {
			return fmt.Errorf("delete organization people cache entry: %w", errorValue)
		}
	}
	return nil
}

func ensureOrganizationPeopleCacheState(ctx context.Context, transaction *sql.Tx, key organizationPeopleCacheKey) error {
	_, errorValue := transaction.ExecContext(ctx, `INSERT INTO organization_people_cache_states(cache_kind, cache_key) VALUES(?, ?) ON CONFLICT(cache_kind, cache_key) DO NOTHING`, string(key.Kind), key.Key)
	if errorValue != nil {
		return fmt.Errorf("ensure organization people cache state: %w", errorValue)
	}
	return nil
}
