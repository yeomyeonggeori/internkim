package admind

import (
	"context"
	"database/sql"
	"strings"
)

func (service *Service) readTaskDefinitions(ctx context.Context) (taskDefinitions, error) {
	database, errorValue := service.openTaskDatabase(ctx)
	if errorValue != nil {
		return taskDefinitions{}, errorValue
	}
	defer database.Close()
	definitions := taskDefinitions{}
	categories, categoryColors, errorValue := readTaskDefinitionValues(ctx, database, "category")
	if errorValue != nil {
		return taskDefinitions{}, errorValue
	}
	types, typeColors, errorValue := readTaskDefinitionValues(ctx, database, "type")
	if errorValue != nil {
		return taskDefinitions{}, errorValue
	}
	sizes, errorValue := readTaskSizeDefinitions(ctx, database)
	if errorValue != nil {
		return taskDefinitions{}, errorValue
	}
	definitions.Categories = categories
	definitions.CategoryColors = categoryColors
	definitions.Types = types
	definitions.TypeColors = typeColors
	definitions.Sizes = sizes
	if len(definitions.Types) == 0 {
		definitions.Types = defaultTaskTypes()
	}
	locale := service.adminLocale()
	if len(definitions.Sizes) == 0 {
		definitions.Sizes = defaultTaskSizeDefinitionsForLocale(locale)
	} else {
		definitions.Sizes = localizedTaskSizeDefinitions(definitions.Sizes, locale)
	}
	return definitions, nil
}

func restoreTaskSizeDefaults(sizes []taskSizeDefinition) []taskSizeDefinition {
	return localizedTaskSizeDefinitions(sizes, "ko")
}

func localizedTaskSizeDefinitions(sizes []taskSizeDefinition, locale string) []taskSizeDefinition {
	koreanDefaultByName := map[string]taskSizeDefinition{}
	for _, definition := range defaultTaskSizeDefinitions() {
		koreanDefaultByName[strings.ToUpper(strings.TrimSpace(definition.Name))] = definition
	}
	defaultByName := map[string]taskSizeDefinition{}
	for _, definition := range defaultTaskSizeDefinitionsForLocale(locale) {
		defaultByName[strings.ToUpper(strings.TrimSpace(definition.Name))] = definition
	}
	result := make([]taskSizeDefinition, 0, len(sizes))
	for _, size := range sizes {
		fallback, hasFallback := defaultByName[strings.ToUpper(strings.TrimSpace(size.Name))]
		if hasFallback {
			if size.DistanceKM <= 0 {
				size.DistanceKM = fallback.DistanceKM
			}
			if size.MaxHours <= 0 {
				size.MaxHours = fallback.MaxHours
			}
			koreanDefault := koreanDefaultByName[strings.ToUpper(strings.TrimSpace(size.Name))]
			size.DevelopmentExample = localizedSizeText(size.DevelopmentExample, koreanDefault.DevelopmentExample, fallback.DevelopmentExample)
			size.OtherExample = localizedSizeText(size.OtherExample, koreanDefault.OtherExample, fallback.OtherExample)
			size.Note = localizedSizeText(size.Note, koreanDefault.Note, fallback.Note)
		}
		size.Score = size.DistanceKM
		size.Label = taskSizeLabelForLocale(size, locale)
		result = append(result, size)
	}
	return result
}

func localizedSizeText(storedText string, koreanDefaultText string, localizedDefaultText string) string {
	trimmedText := strings.TrimSpace(storedText)
	if trimmedText == "" || trimmedText == strings.TrimSpace(koreanDefaultText) {
		return localizedDefaultText
	}
	return trimmedText
}

func readTaskDefinitionValues(ctx context.Context, database *sql.DB, kind string) ([]string, map[string]string, error) {
	rows, errorValue := database.QueryContext(ctx, "SELECT value, color FROM flow_definitions WHERE kind = ? ORDER BY position, value", kind)
	if errorValue != nil {
		return nil, nil, errorValue
	}
	defer rows.Close()
	values := []string{}
	colors := map[string]string{}
	for rows.Next() {
		var value string
		var color string
		if errorValue := rows.Scan(&value, &color); errorValue != nil {
			return nil, nil, errorValue
		}
		values = append(values, value)
		if strings.TrimSpace(color) != "" {
			colors[value] = color
		}
	}
	return values, colors, rows.Err()
}

func readTaskSizeDefinitions(ctx context.Context, database *sql.DB) ([]taskSizeDefinition, error) {
	rows, errorValue := database.QueryContext(ctx, `
SELECT name, distance_km, max_hours, development_example, other_example, note
FROM flow_size_definitions
ORDER BY position, name`)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	sizes := []taskSizeDefinition{}
	for rows.Next() {
		var size taskSizeDefinition
		if errorValue := rows.Scan(&size.Name, &size.DistanceKM, &size.MaxHours, &size.DevelopmentExample, &size.OtherExample, &size.Note); errorValue != nil {
			return nil, errorValue
		}
		size.Score = size.DistanceKM
		size.Label = taskSizeLabel(size)
		sizes = append(sizes, size)
	}
	return sizes, rows.Err()
}

func (service *Service) writeTaskDefinitions(ctx context.Context, definitions taskDefinitions) error {
	database, errorValue := service.openTaskDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := replaceTaskDefinitionKind(ctx, transaction, "category", definitions.Categories, definitions.CategoryColors); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := replaceTaskDefinitionKind(ctx, transaction, "type", definitions.Types, definitions.TypeColors); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := replaceTaskSizeDefinitions(ctx, transaction, definitions.Sizes); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := incrementTaskSummarySourceRevisions(ctx, transaction, []taskSummarySourceKey{taskSummaryDefinitionsSourceKey()}); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	return transaction.Commit()
}

func replaceTaskDefinitionKind(ctx context.Context, transaction *sql.Tx, kind string, values []string, colors map[string]string) error {
	if _, errorValue := transaction.ExecContext(ctx, "DELETE FROM flow_definitions WHERE kind = ?", kind); errorValue != nil {
		return errorValue
	}
	for index, value := range values {
		if _, errorValue := transaction.ExecContext(ctx, "INSERT INTO flow_definitions(kind, value, position, color) VALUES(?, ?, ?, ?)", kind, value, index, strings.TrimSpace(colors[value])); errorValue != nil {
			return errorValue
		}
	}
	_, errorValue := transaction.ExecContext(ctx, "INSERT INTO flow_definition_meta(kind, initialized) VALUES(?, 1) ON CONFLICT(kind) DO UPDATE SET initialized = 1", kind)
	return errorValue
}

func replaceTaskSizeDefinitions(ctx context.Context, transaction interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, sizes []taskSizeDefinition) error {
	if _, errorValue := transaction.ExecContext(ctx, "DELETE FROM flow_size_definitions"); errorValue != nil {
		return errorValue
	}
	for index, size := range sizes {
		if _, errorValue := transaction.ExecContext(ctx, `
INSERT INTO flow_size_definitions(name, distance_km, max_hours, development_example, other_example, note, position)
VALUES(?, ?, ?, ?, ?, ?, ?)`,
			size.Name,
			size.DistanceKM,
			size.MaxHours,
			size.DevelopmentExample,
			size.OtherExample,
			size.Note,
			index,
		); errorValue != nil {
			return errorValue
		}
	}
	_, errorValue := transaction.ExecContext(ctx, "INSERT INTO flow_definition_meta(kind, initialized) VALUES(?, 1) ON CONFLICT(kind) DO UPDATE SET initialized = 1", "size")
	return errorValue
}
