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
	categories, errorValue := readFlowDefinitionValues(ctx, database, "category")
	if errorValue != nil {
		return flowDefinitions{}, errorValue
	}
	types, errorValue := readFlowDefinitionValues(ctx, database, "type")
	if errorValue != nil {
		return flowDefinitions{}, errorValue
	}
	sizes, errorValue := readFlowSizeDefinitions(ctx, database)
	if errorValue != nil {
		return flowDefinitions{}, errorValue
	}
	definitions.Categories = categories
	definitions.Types = types
	definitions.Sizes = sizes
	if len(definitions.Types) == 0 {
		definitions.Types = defaultFlowTypes()
	}
	if len(definitions.Sizes) == 0 {
		definitions.Sizes = defaultFlowSizeDefinitions()
	} else {
		definitions.Sizes = restoreFlowSizeDefaults(definitions.Sizes)
	}
	return definitions, nil
}

func restoreFlowSizeDefaults(sizes []flowSizeDefinition) []flowSizeDefinition {
	defaultByName := map[string]flowSizeDefinition{}
	for _, definition := range defaultFlowSizeDefinitions() {
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
			if strings.TrimSpace(size.DevelopmentExample) == "" {
				size.DevelopmentExample = fallback.DevelopmentExample
			}
			if strings.TrimSpace(size.OtherExample) == "" {
				size.OtherExample = fallback.OtherExample
			}
			if strings.TrimSpace(size.Note) == "" {
				size.Note = fallback.Note
			}
		}
		size.Score = size.DistanceKM
		size.Label = flowSizeLabel(size)
		result = append(result, size)
	}
	return result
}

func readFlowDefinitionValues(ctx context.Context, database *sql.DB, kind string) ([]string, error) {
	rows, errorValue := database.QueryContext(ctx, "SELECT value FROM flow_definitions WHERE kind = ? ORDER BY position, value", kind)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	values := []string{}
	for rows.Next() {
		var value string
		if errorValue := rows.Scan(&value); errorValue != nil {
			return nil, errorValue
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func readFlowSizeDefinitions(ctx context.Context, database *sql.DB) ([]flowSizeDefinition, error) {
	rows, errorValue := database.QueryContext(ctx, `
SELECT name, distance_km, max_hours, development_example, other_example, note
FROM flow_size_definitions
ORDER BY position, name`)
	if errorValue != nil {
		return nil, errorValue
	}
	defer rows.Close()
	sizes := []flowSizeDefinition{}
	for rows.Next() {
		var size flowSizeDefinition
		if errorValue := rows.Scan(&size.Name, &size.DistanceKM, &size.MaxHours, &size.DevelopmentExample, &size.OtherExample, &size.Note); errorValue != nil {
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
	if errorValue := replaceFlowDefinitionKind(ctx, transaction, "category", definitions.Categories); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := replaceFlowDefinitionKind(ctx, transaction, "type", definitions.Types); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	if errorValue := replaceFlowSizeDefinitions(ctx, transaction, definitions.Sizes); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	definitionsKey := flowSummarySourceKey{Kind: flowSummarySourceDefinitions, Key: "global"}
	if errorValue := incrementFlowSummarySourceRevisions(ctx, transaction, []flowSummarySourceKey{definitionsKey}); errorValue != nil {
		_ = transaction.Rollback()
		return errorValue
	}
	return transaction.Commit()
}

func replaceFlowDefinitionKind(ctx context.Context, transaction *sql.Tx, kind string, values []string) error {
	if _, errorValue := transaction.ExecContext(ctx, "DELETE FROM flow_definitions WHERE kind = ?", kind); errorValue != nil {
		return errorValue
	}
	for index, value := range values {
		if _, errorValue := transaction.ExecContext(ctx, "INSERT INTO flow_definitions(kind, value, position) VALUES(?, ?, ?)", kind, value, index); errorValue != nil {
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
