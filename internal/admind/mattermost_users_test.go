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
