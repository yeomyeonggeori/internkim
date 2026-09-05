package tasksize

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

//go:embed definitions.json
var definitionsDocument []byte

type Definition struct {
	Name       string        `json:"name"`
	DistanceKM int           `json:"distanceKm"`
	MaxHours   int           `json:"maxHours"`
	Score      int           `json:"score"`
	Korean     LocalizedText `json:"ko"`
	English    LocalizedText `json:"en"`
}

type LocalizedText struct {
	DevelopmentExample string `json:"developmentExample"`
	OtherExample       string `json:"otherExample"`
	Note               string `json:"note"`
}

type definitionsDocumentShape struct {
	Sizes []Definition `json:"sizes"`
}

var canonicalDefinitions = loadDefinitions()

func Definitions() []Definition {
	return slices.Clone(canonicalDefinitions)
}

func loadDefinitions() []Definition {
	var document definitionsDocumentShape
	decoder := json.NewDecoder(bytes.NewReader(definitionsDocument))
	decoder.DisallowUnknownFields()
	if errorValue := decoder.Decode(&document); errorValue != nil {
		panic(fmt.Sprintf("invalid task-size definitions: %v", errorValue))
	}
	if errorValue := validateDefinitions(document.Sizes); errorValue != nil {
		panic(errorValue)
	}
	return document.Sizes
}

func validateDefinitions(definitions []Definition) error {
	if len(definitions) == 0 {
		return fmt.Errorf("task-size definitions are empty")
	}
	seenNames := map[string]bool{}
	previousMaxHours := 0
	for _, definition := range definitions {
		if definition.Name == "" || seenNames[definition.Name] {
			return fmt.Errorf("task-size definition name is empty or duplicated: %q", definition.Name)
		}
		if definition.DistanceKM <= 0 || definition.MaxHours <= 0 || definition.Score <= 0 {
			return fmt.Errorf("task-size definition %q has non-positive values", definition.Name)
		}
		if definition.MaxHours <= previousMaxHours {
			return fmt.Errorf("task-size definition %q does not increase maxHours", definition.Name)
		}
		if hasEmptyLocalizedText(definition.Korean) || hasEmptyLocalizedText(definition.English) {
			return fmt.Errorf("task-size definition %q has incomplete localized text", definition.Name)
		}
		seenNames[definition.Name] = true
		previousMaxHours = definition.MaxHours
	}
	return nil
}

func hasEmptyLocalizedText(text LocalizedText) bool {
	return strings.TrimSpace(text.DevelopmentExample) == "" || strings.TrimSpace(text.OtherExample) == "" || strings.TrimSpace(text.Note) == ""
}
