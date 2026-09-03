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

func TestAnExactNameWinsOverALongerNameThatContainsIt(t *testing.T) {
	resolution := resolveHint("박예시", directoryPeopleFixture(), nil)

	if resolution.Outcome != hintResolved || resolution.Match.MemberID != "person-2" {
		t.Fatalf("a whole name is that person, got %+v", resolution)
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
