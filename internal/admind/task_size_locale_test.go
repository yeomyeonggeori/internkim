package admind

import "testing"

func TestTaskSizeDefinitionsSpeakTheAdminLocale(t *testing.T) {
	english := defaultTaskSizeDefinitionsForLocale("en")

	if english[0].Name != "XS" {
		t.Fatalf("first size = %q, want XS", english[0].Name)
	}
	if english[0].DevelopmentExample != "Trivial change" {
		t.Fatalf("development example = %q, want the English default", english[0].DevelopmentExample)
	}
	if english[0].Note != "Done in a moment" {
		t.Fatalf("note = %q, want the English default", english[0].Note)
	}
	if english[0].Label != "1km · max 1h" {
		t.Fatalf("label = %q, want the English label", english[0].Label)
	}
	if english[0].DistanceKM != 1 || english[0].MaxHours != 1 {
		t.Fatalf("distance/hours = %d/%d, want the rubric's numbers", english[0].DistanceKM, english[0].MaxHours)
	}

	korean := defaultTaskSizeDefinitionsForLocale("ko")

	if korean[0].Note != "잠깐이면 끝낼 것" {
		t.Fatalf("note = %q, want the Korean default", korean[0].Note)
	}
	if korean[0].Label != "1km · 최대 1h" {
		t.Fatalf("label = %q, want the Korean label", korean[0].Label)
	}
}
