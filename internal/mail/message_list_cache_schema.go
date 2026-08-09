package mail

import (
	"context"
	"database/sql"
)

func EnsureMessageListCacheSchema(ctx context.Context, database *sql.DB) error {
	hasListSchema, errorValue := cacheTableHasColumns(ctx, database, "mail_message_list_cache", "before_uid", "page_limit", "next_cursor")
	if errorValue != nil {
		return errorValue
	}
	hasItemSchema, errorValue := cacheTableHasColumns(ctx, database, "mail_message_list_cache_item", "page_limit", "uid", "position")
	if errorValue != nil {
		return errorValue
	}
	if !hasListSchema || !hasItemSchema {
		if _, errorValue := database.ExecContext(ctx, `DROP TABLE IF EXISTS mail_message_list_cache_item`); errorValue != nil {
			return errorValue
		}
		if _, errorValue := database.ExecContext(ctx, `DROP TABLE IF EXISTS mail_message_list_cache`); errorValue != nil {
			return errorValue
		}
	}
	_, errorValue = database.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS mail_message_list_cache (
	actor_email TEXT NOT NULL,
	mailbox TEXT NOT NULL,
		query TEXT NOT NULL,
		before_uid INTEGER NOT NULL,
		page_limit INTEGER NOT NULL,
		next_cursor TEXT NOT NULL,
		updated_at TEXT NOT NULL,
		PRIMARY KEY(actor_email, mailbox, query, before_uid, page_limit)
	);
	CREATE TABLE IF NOT EXISTS mail_message_list_cache_item (
		actor_email TEXT NOT NULL,
		mailbox TEXT NOT NULL,
		query TEXT NOT NULL,
		before_uid INTEGER NOT NULL,
		page_limit INTEGER NOT NULL,
		uid INTEGER NOT NULL,
		position INTEGER NOT NULL,
		updated_at TEXT NOT NULL,
		PRIMARY KEY(actor_email, mailbox, query, before_uid, page_limit, uid)
	)`)
	return errorValue
}

func cacheTableHasColumns(ctx context.Context, database *sql.DB, tableName string, columnNames ...string) (bool, error) {
	rows, errorValue := database.QueryContext(ctx, `PRAGMA table_info(`+tableName+`)`)
	if errorValue != nil {
		return false, errorValue
	}
	defer rows.Close()
	requiredColumns := make(map[string]bool, len(columnNames))
	for _, columnName := range columnNames {
		requiredColumns[columnName] = false
	}
	hasColumns := false
	for rows.Next() {
		hasColumns = true
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var primaryKey int
		if errorValue := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); errorValue != nil {
			return false, errorValue
		}
		if _, ok := requiredColumns[name]; ok {
			requiredColumns[name] = true
		}
	}
	if errorValue := rows.Err(); errorValue != nil {
		return false, errorValue
	}
	if !hasColumns {
		return true, nil
	}
	for _, hasColumn := range requiredColumns {
		if !hasColumn {
			return false, nil
		}
	}
	return true, nil
}
