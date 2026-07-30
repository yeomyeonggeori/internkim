package admind

import (
	"context"
	"database/sql"
	"strings"
)

func (service *Service) readFlowDefinitions(ctx context.Context) (flowDefinitions, error) {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return flowDefinitions{}, errorValue
	}
	defer database.Close()
	definitions := flowDefinitions{}
	categories, categoryColors, errorValue := readFlowDefinitionValues(ctx, database, "category")
	if errorValue != nil {
		return flowDefinitions{}, errorValue
	}
	types, typeColors, errorValue := readFlowDefinitionValues(ctx, database, "type")
	if errorValue != nil {
		return flowDefinitions{}, errorValue
	}
	sizes, errorValue := readFlowSizeDefinitions(ctx, database)
	if errorValue != nil {
		return flowDefinitions{}, errorValue
	}
	definitions.Categories = categories
	definitions.CategoryColors = categoryColors
	definitions.Types = types
	definitions.TypeColors = typeColors
	definitions.Sizes = sizes
	if len(definitions.Types) == 0 {
		definitions.Types = defaultFlowTypes()
	}
	locale := service.adminLocale()
	if len(definitions.Sizes) == 0 {
		definitions.Sizes = defaultFlowSizeDefinitionsForLocale(locale)
	} else {
		definitions.Sizes = localizedFlowSizeDefinitions(definitions.Sizes, locale)
	}
	return definitions, nil
}

func restoreFlowSizeDefaults(sizes []flowSizeDefinition) []flowSizeDefinition {
	return localizedFlowSizeDefinitions(sizes, "ko")
}

func localizedFlowSizeDefinitions(sizes []flowSizeDefinition, locale string) []flowSizeDefinition {
	koreanDefaultByName := map[string]flowSizeDefinition{}
	for _, definition := range defaultFlowSizeDefinitions() {
		koreanDefaultByName[strings.ToUpper(strings.TrimSpace(definition.Name))] = definition
	}
	defaultByName := map[string]flowSizeDefinition{}
	for _, definition := range defaultFlowSizeDefinitionsForLocale(locale) {
		defaultByName[strings.ToUpper(strings.TrimSpace(definition.Name))] = definition
	}
	result := make([]flowSizeDefinition, 0, len(sizes))
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
		size.Label = flowSizeLabelForLocale(size, locale)
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

func readFlowDefinitionValues(ctx context.Context, database *sql.DB, kind string) ([]string, map[string]string, error) {
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

func readFlowSizeDefinitions(ctx context.Context, database *sql.DB) ([]flowSizeDefinition, error) {
	rows, errorValue := database.QueryContext(ctx, `
SELECT name, distance_km, max_hours, development_example, other_example, note, color
FROM flow_size_definitions
ORDER BY position, name`)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	sizes := []flowSizeDefinition{}
	for rows.Next() {
		var size flowSizeDefinition
		if errorValue := rows.Scan(&size.Name, &size.DistanceKM, &size.MaxHours, &size.DevelopmentExample, &size.OtherExample, &size.Note, &size.Color); errorValue != nil {
			return nil, errorValue
		}
		size.Score = size.DistanceKM
		size.Label = flowSizeLabel(size)
		sizes = append(sizes, size)
	}
	return sizes, rows.Err()
}

func (service *Service) writeFlowDefinitions(ctx context.Context, definitions flowDefinitions) error {
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		return errorValue
	}
	defer database.Close()
	transaction, errorValue := database.BeginTx(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := replaceFlowDefinitionKind(ctx, transaction, "category", definitions.Categories, definitions.CategoryColors); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := replaceFlowDefinitionKind(ctx, transaction, "type", definitions.Types, definitions.TypeColors); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := replaceFlowSizeDefinitions(ctx, transaction, definitions.Sizes); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := incrementFlowSummarySourceRevisions(ctx, transaction, []flowSummarySourceKey{flowSummaryDefinitionsSourceKey()}); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	return transaction.Commit()
}

func replaceFlowDefinitionKind(ctx context.Context, transaction *sql.Tx, kind string, values []string, colors map[string]string) error {
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

func replaceFlowSizeDefinitions(ctx context.Context, transaction interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, sizes []flowSizeDefinition) error {
	if _, errorValue := transaction.ExecContext(ctx, "DELETE FROM flow_size_definitions"); errorValue != nil {
		return errorValue
	}
	for index, size := range sizes {
		if _, errorValue := transaction.ExecContext(ctx, `
INSERT INTO flow_size_definitions(name, distance_km, max_hours, development_example, other_example, note, position, color)
VALUES(?, ?, ?, ?, ?, ?, ?, ?)`,
			size.Name,
			size.DistanceKM,
			size.MaxHours,
			size.DevelopmentExample,
			size.OtherExample,
			size.Note,
			index,
			strings.TrimSpace(size.Color),
		); errorValue != nil {
			return errorValue
		}
	}
	_, errorValue := transaction.ExecContext(ctx, "INSERT INTO flow_definition_meta(kind, initialized) VALUES(?, 1) ON CONFLICT(kind) DO UPDATE SET initialized = 1", "size")
	return errorValue
}
