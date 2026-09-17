package capabilityd

import "testing"

func directoryPeopleFixture() []directoryPerson {
	return []directoryPerson{
		{MemberID: "person-1", Email: "kimyesi@example.com", Name: "김예시"},
		{MemberID: "person-2", Email: "parkyesi@example.com", Name: "박예시"},
		{MemberID: "person-3", Email: "parkyesiyeon@example.com", Name: "박예시연"},
	}
}

func TestAnIdentifierResolvesAndANearOneDoesNot(t *testing.T) {
	people := directoryPeopleFixture()

	if resolution := resolveHint("person-2", people, nil); resolution.Outcome != hintResolved || resolution.Match.MemberID != "person-2" {
		t.Fatalf("an exact identifier is taken as given, got %+v", resolution)
	}
	if resolution := resolveHint("person-9", people, nil); resolution.Outcome != hintNotFound {
		t.Fatalf("an identifier a character off was invented, not mistyped, got %+v", resolution)
	}
}

func TestANameThatBeginsAnotherNameIsAQuestion(t *testing.T) {
	resolution := resolveHint("박예시", directoryPeopleFixture(), nil)

	if resolution.Outcome != hintAmbiguous || len(resolution.Candidates) != 2 {
		t.Fatalf("a name two people answer to is never picked here, got %+v", resolution)
	}
}

func TestANameInsideSeveralNamesOffersOnlyThose(t *testing.T) {
	resolution := resolveHint("박", directoryPeopleFixture(), nil)

	if resolution.Outcome != hintAmbiguous || len(resolution.Candidates) != 2 {
		t.Fatalf("a name several people answer to is a question, got %+v", resolution)
	}
}

func TestANameRememberedWrongIsProposedRatherThanGuessed(t *testing.T) {
	resolution := resolveHint("김여시", directoryPeopleFixture(), nil)

	if resolution.Outcome != hintApproximate {
		t.Fatalf("a name a character off is a typo to propose, got %+v", resolution)
	}
	if len(resolution.Candidates) != 1 || resolution.Candidates[0].Name != "김예시" {
		t.Fatalf("only the near name belongs in the question, got %+v", resolution.Candidates)
	}
}

func TestANameNothingComesCloseToNamesNothing(t *testing.T) {
	resolution := resolveHint("최견본", directoryPeopleFixture(), nil)

	if resolution.Outcome != hintNotFound || len(resolution.Candidates) != 0 {
		t.Fatalf("nothing close is nothing to offer, got %+v", resolution)
	}
}

func TestANameInEitherOrderIsTheSamePerson(t *testing.T) {
	people := []directoryPerson{
		{MemberID: "person-lee", Email: "sample@example.com", Name: "샘플 이"},
		{MemberID: "person-smith", Email: "smith@example.com", Name: "John Michael Smith"},
	}
	for hint, memberID := range map[string]string{
		"이샘플":                "person-lee",
		"샘플 이":               "person-lee",
		"샘플이":                "person-lee",
		"Smith John Michael": "person-smith",
		"john michael smith": "person-smith",
		"John Smith":         "person-smith",
		"Smith John":         "person-smith",
	} {
		resolution := resolveHint(hint, people, nil)
		if resolution.Outcome != hintResolved || resolution.Match.MemberID != memberID {
			t.Errorf("%q is written in one of the orders a name has, got %+v", hint, resolution)
		}
	}
}

func TestANameWithAndWithoutAMiddleIsAQuestion(t *testing.T) {
	people := []directoryPerson{
		{MemberID: "person-plain", Email: "plain@example.com", Name: "John Smith"},
		{MemberID: "person-middle", Email: "middle@example.com", Name: "John Michael Smith"},
	}
	for _, hint := range []string{"Smith John", "John Smith", "Smith"} {
		if resolution := resolveHint(hint, people, nil); resolution.Outcome != hintAmbiguous || len(resolution.Candidates) != 2 {
			t.Errorf("%q could be either of them, got %+v", hint, resolution)
		}
	}
	if resolution := resolveHint("Michael", people, nil); resolution.Outcome != hintResolved || resolution.Match.MemberID != "person-middle" {
		t.Fatalf("a name only one person answers to is that person, got %+v", resolution)
	}
}

func TestTwoPeopleWithOneNameAreAQuestion(t *testing.T) {
	people := []directoryPerson{
		{MemberID: "person-work", Email: "mohyeong@example.com", Name: "이모형(dawn.kim)"},
		{MemberID: "person-personal", Email: "mohyeong2468@example.com", Name: "모형 이"},
		{MemberID: "person-other", Email: "other@example.com", Name: "예시 김"},
	}
	for _, hint := range []string{"이모형", "모형", "모형 이"} {
		resolution := resolveHint(hint, people, nil)
		if resolution.Outcome != hintAmbiguous || len(resolution.Candidates) != 2 {
			t.Errorf("%q names two people and nothing here may pick one, got %+v", hint, resolution)
		}
	}
	if resolution := resolveHint("mohyeong2468@example.com", people, nil); resolution.Outcome != hintResolved || resolution.Match.MemberID != "person-personal" {
		t.Fatalf("an address tells two people with one name apart, got %+v", resolution)
	}
	if resolution := resolveHint("예시", people, nil); resolution.Outcome != hintResolved || resolution.Match.MemberID != "person-other" {
		t.Fatalf("a name one person answers to is that person, got %+v", resolution)
	}
}
