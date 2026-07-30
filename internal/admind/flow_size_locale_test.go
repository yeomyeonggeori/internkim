package admind

import "testing"

func TestLocalizedFlowSizeDefinitionsTranslatesSeededText(t *testing.T) {
	sizes := localizedFlowSizeDefinitions(defaultFlowSizeDefinitions(), "en")

	extraSmall := sizes[0]
	if extraSmall.Name != "XS" {
		t.Fatalf("first size = %q, want XS", extraSmall.Name)
	}
	if extraSmall.DevelopmentExample != "Trivial change" {
		t.Fatalf("development example = %q, want the English default", extraSmall.DevelopmentExample)
	}
	if extraSmall.Note != "Done in a moment" {
		t.Fatalf("note = %q, want the English default", extraSmall.Note)
	}
	if extraSmall.Label != "1km · max 1h" {
		t.Fatalf("label = %q, want the English label", extraSmall.Label)
	}
	if extraSmall.DistanceKM != 1 || extraSmall.MaxHours != 1 {
		t.Fatalf("distance/hours = %d/%d, want the stored numbers", extraSmall.DistanceKM, extraSmall.MaxHours)
	}
}

func TestLocalizedFlowSizeDefinitionsKeepsCustomText(t *testing.T) {
	sizes := defaultFlowSizeDefinitions()
	sizes[0].Note = "우리 팀 기준"

	localized := localizedFlowSizeDefinitions(sizes, "en")

	if localized[0].Note != "우리 팀 기준" {
		t.Fatalf("note = %q, want the customized text kept", localized[0].Note)
	}
	if localized[0].DevelopmentExample != "Trivial change" {
		t.Fatalf("development example = %q, want the English default", localized[0].DevelopmentExample)
	}
}

func TestLocalizedFlowSizeDefinitionsKeepsKoreanForKoreanLocale(t *testing.T) {
	sizes := localizedFlowSizeDefinitions(defaultFlowSizeDefinitions(), "ko")

	if sizes[0].Note != "잠깐이면 끝낼 것" {
		t.Fatalf("note = %q, want the Korean default", sizes[0].Note)
	}
	if sizes[0].Label != "1km · 최대 1h" {
		t.Fatalf("label = %q, want the Korean label", sizes[0].Label)
	}
}
