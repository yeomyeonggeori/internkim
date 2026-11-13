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

func TestAWholeNameIsTakenEvenWhenAnotherNameBeginsWithIt(t *testing.T) {
	resolution := resolveHint("박예시", directoryPeopleFixture(), nil)

	if resolution.Outcome != hintResolved || resolution.Match.MemberID != "person-2" {
		t.Fatalf("the person whose whole name was given is the one meant, got %+v", resolution)
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

func TestTwoPeopleAPartOfANameReachesAlikeAreAQuestion(t *testing.T) {
	people := []directoryPerson{
		{MemberID: "person-work", Email: "sample@example.com", Name: "이샘플(work)"},
		{MemberID: "person-personal", Email: "sample2468@example.com", Name: "샘플 이"},
		{MemberID: "person-other", Email: "other@example.com", Name: "예시 김"},
	}
	if resolution := resolveHint("샘플", people, nil); resolution.Outcome != hintAmbiguous || len(resolution.Candidates) != 2 {
		t.Errorf("a part of both names is a question, got %+v", resolution)
	}
	if resolution := resolveHint("이샘플", people, nil); resolution.Outcome != hintResolved || resolution.Match.MemberID != "person-personal" {
		t.Errorf("a whole name outranks a name that merely contains it, got %+v", resolution)
	}
	if resolution := resolveHint("sample2468@example.com", people, nil); resolution.Outcome != hintResolved || resolution.Match.MemberID != "person-personal" {
		t.Fatalf("an address tells two people with one name apart, got %+v", resolution)
	}
}

func TestAChannelIsReachedOnlyByItsWholeName(t *testing.T) {
	channels := []chatdChannel{{ChannelID: "channel-1", Name: "general"}, {ChannelID: "channel-2", Name: "general-talk"}}

	if resolution := resolveHint("#General", channels, nil); resolution.Outcome != hintResolved || resolution.Match.ChannelID != "channel-1" {
		t.Fatalf("a channel's whole name reaches it, got %+v", resolution)
	}
	if resolution := resolveHint("talk", channels, nil); resolution.Outcome == hintResolved {
		t.Fatalf("a part of a channel name is never taken as the channel, got %+v", resolution)
	}
}
