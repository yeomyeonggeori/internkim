package tasksize

import "testing"

func TestDefinitionsMatchTheCanonicalTaskSizeRubric(t *testing.T) {
	definitions := Definitions()
	wantNames := []string{"XS", "S", "M", "L", "XL", "XXL"}
	wantDistanceKM := []int{1, 2, 3, 5, 8, 13}
	wantMaxHours := []int{1, 2, 8, 16, 32, 128}

	if len(definitions) != len(wantNames) {
		t.Fatalf("definition count = %d, want %d", len(definitions), len(wantNames))
	}
	for index, definition := range definitions {
		if definition.Name != wantNames[index] || definition.DistanceKM != wantDistanceKM[index] || definition.MaxHours != wantMaxHours[index] || definition.Score != wantDistanceKM[index] {
			t.Fatalf("definition %d = %+v, want name/distance/hours/score %s/%d/%d/%d", index, definition, wantNames[index], wantDistanceKM[index], wantMaxHours[index], wantDistanceKM[index])
		}
	}
}

func TestDefinitionsReturnsAnIndependentCopy(t *testing.T) {
	definitions := Definitions()
	definitions[0].Name = "changed"
	if Definitions()[0].Name == "changed" {
		t.Fatal("Definitions returned mutable canonical state")
	}
}

func TestValidateDefinitionsRequiresIncreasingMaximumHours(t *testing.T) {
	definitions := Definitions()
	definitions[1].MaxHours = definitions[0].MaxHours
	if errorValue := validateDefinitions(definitions); errorValue == nil {
		t.Fatal("validateDefinitions accepted non-increasing maxHours")
	}
}
