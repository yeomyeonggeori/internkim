package admind

import "testing"

func TestAddMattermostNameFieldsSplitsKoreanName(t *testing.T) {
	body := map[string]string{}
	addMattermostNameFields(body, "김민수")

	if body["first_name"] != "민수" {
		t.Errorf("first_name = %q; want %q", body["first_name"], "민수")
	}
	if body["last_name"] != "김" {
		t.Errorf("last_name = %q; want %q", body["last_name"], "김")
	}
	if body["nickname"] != "김민수" {
		t.Errorf("nickname = %q; want %q", body["nickname"], "김민수")
	}
}

func TestAddMattermostNameFieldsSplitsEnglishName(t *testing.T) {
	body := map[string]string{}
	addMattermostNameFields(body, "Ada Lovelace")

	if body["first_name"] != "Ada" {
		t.Errorf("first_name = %q; want %q", body["first_name"], "Ada")
	}
	if body["last_name"] != "Lovelace" {
		t.Errorf("last_name = %q; want %q", body["last_name"], "Lovelace")
	}
	if body["nickname"] != "Ada Lovelace" {
		t.Errorf("nickname = %q; want %q", body["nickname"], "Ada Lovelace")
	}
}

func TestAddMattermostNameFieldsSingleTokenOmitsLastName(t *testing.T) {
	body := map[string]string{}
	addMattermostNameFields(body, "Madonna")

	if body["first_name"] != "Madonna" {
		t.Errorf("first_name = %q; want %q", body["first_name"], "Madonna")
	}
	if _, hasLastName := body["last_name"]; hasLastName {
		t.Errorf("last_name should be absent for single-token name; got %q", body["last_name"])
	}
	if body["nickname"] != "Madonna" {
		t.Errorf("nickname = %q; want %q", body["nickname"], "Madonna")
	}
}

func TestAddMattermostNameFieldsEmptyNameWritesNothing(t *testing.T) {
	body := map[string]string{}
	addMattermostNameFields(body, "   ")

	if len(body) != 0 {
		t.Errorf("empty name should write no fields; got %#v", body)
	}
}

func TestMattermostSyncedPersonCirclesUsesStaffAndChannelMembership(t *testing.T) {
	person := map[string]any{
		"emails": []any{"minsu@example.com", "other@example.com"},
	}
	circles := mattermostSyncedPersonCircles(person, map[string]map[string]bool{
		"finance":        {"minsu@example.com": true},
		"representative": {"someone@example.com": true},
	})

	if !containsMattermostTestString(circles, "staff") || !containsMattermostTestString(circles, "finance") {
		t.Fatalf("expected staff and finance circles, got %+v", circles)
	}
	if containsMattermostTestString(circles, "representative") {
		t.Fatalf("expected non-member representative circle omitted, got %+v", circles)
	}
}

func TestMattermostSyncedPersonCirclesKeepsAdminFromPolicy(t *testing.T) {
	person := map[string]any{
		"emails":  []any{"owner@example.com"},
		"isAdmin": true,
	}
	circles := mattermostSyncedPersonCircles(person, map[string]map[string]bool{})

	if !containsMattermostTestString(circles, "staff") || !containsMattermostTestString(circles, "admin") {
		t.Fatalf("expected staff and admin circles, got %+v", circles)
	}
}

func TestDefaultMattermostCircleChannels(t *testing.T) {
	channels := defaultMattermostCircleChannels()
	for circleID, expectedChannelName := range map[string]string{
		"admin":          "circle-admin",
		"c-level":        "circle-c-level",
		"representative": "circle-representative",
	} {
		if channels[circleID] != expectedChannelName {
			t.Fatalf("expected %s channel %q, got %+v", circleID, expectedChannelName, channels)
		}
	}
}

func containsMattermostTestString(values []string, expectedValue string) bool {
	for _, value := range values {
		if value == expectedValue {
			return true
		}
	}
	return false
}
