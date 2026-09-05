package admind

import (
	"testing"

	"gitlab.com/eastriver/internkim/internal/tasksize"
)

func TestAdminTaskSizeDefinitionsMatchCanonicalSource(t *testing.T) {
	canonicalDefinitions := tasksize.Definitions()
	locales := []struct {
		name string
		text func(tasksize.Definition) tasksize.LocalizedText
	}{
		{name: "ko", text: func(definition tasksize.Definition) tasksize.LocalizedText { return definition.Korean }},
		{name: "en", text: func(definition tasksize.Definition) tasksize.LocalizedText { return definition.English }},
	}

	for _, locale := range locales {
		t.Run(locale.name, func(t *testing.T) {
			adminDefinitions := defaultTaskSizeDefinitionsForLocale(locale.name)
			if len(adminDefinitions) != len(canonicalDefinitions) {
				t.Fatalf("definition count = %d, want %d", len(adminDefinitions), len(canonicalDefinitions))
			}
			for index, canonicalDefinition := range canonicalDefinitions {
				adminDefinition := adminDefinitions[index]
				localizedText := locale.text(canonicalDefinition)
				if adminDefinition.Name != canonicalDefinition.Name ||
					adminDefinition.DistanceKM != canonicalDefinition.DistanceKM ||
					adminDefinition.MaxHours != canonicalDefinition.MaxHours ||
					adminDefinition.Score != canonicalDefinition.Score ||
					adminDefinition.DevelopmentExample != localizedText.DevelopmentExample ||
					adminDefinition.OtherExample != localizedText.OtherExample ||
					adminDefinition.Note != localizedText.Note {
					t.Fatalf("definition %d = %+v, want canonical %q in %s", index, adminDefinition, canonicalDefinition.Name, locale.name)
				}
			}
		})
	}
}
