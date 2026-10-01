package admind

import (
	"fmt"

	"github.com/yeomyeonggeori/internkim/internal/tasksize"
)

func defaultTaskSizeDefinitions() []taskSizeDefinition {
	return defaultTaskSizeDefinitionsForLocale("ko")
}

func defaultTaskSizeDefinitionsForLocale(locale string) []taskSizeDefinition {
	definitions := tasksize.Definitions()
	adminDefinitions := make([]taskSizeDefinition, 0, len(definitions))
	for _, definition := range definitions {
		localizedText := definition.Korean
		if normalizeAdminLocale(locale) == "en" {
			localizedText = definition.English
		}
		adminDefinition := taskSizeDefinition{
			Name:               definition.Name,
			DistanceKM:         definition.DistanceKM,
			MaxHours:           definition.MaxHours,
			DevelopmentExample: localizedText.DevelopmentExample,
			OtherExample:       localizedText.OtherExample,
			Note:               localizedText.Note,
			Score:              definition.Score,
		}
		adminDefinition.Label = taskSizeLabelForLocale(adminDefinition, locale)
		adminDefinitions = append(adminDefinitions, adminDefinition)
	}
	return adminDefinitions
}

func taskSizeLabelForLocale(size taskSizeDefinition, locale string) string {
	if normalizeAdminLocale(locale) == "en" {
		return fmt.Sprintf("%dkm · max %dh", size.DistanceKM, size.MaxHours)
	}
	return fmt.Sprintf("%dkm · 최대 %dh", size.DistanceKM, size.MaxHours)
}

func containsTaskSize(values []taskSizeDefinition, target string) bool {
	for _, value := range values {
		if value.Name == target {
			return true
		}
	}
	return false
}
