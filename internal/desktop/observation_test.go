package desktop

import (
	"slices"
	"strings"
	"testing"
)

func argumentsOf(t *testing.T, kind Kind, program string, arguments ...string) []string {
	t.Helper()
	call, errorValue := NewCall(kind, program, arguments, "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return call.Arguments
}

func TestASnapshotThatChoosesNoSizeIsAskedForInItsCompactInteractiveForm(t *testing.T) {
	for program, want := range map[string][]string{
		"agent-desktop": {"snapshot", "--app", "Finder", "-i", "--compact"},
		"agent-browser": {"snapshot", "-i", "--json", "--headed"},
	} {
		arguments := []string{"snapshot"}
		if program == DefaultProgram {
			arguments = []string{"snapshot", "--app", "Finder"}
		}
		if got := argumentsOf(t, KindRead, program, arguments...); !slices.Equal(got, want) {
			t.Errorf("%s was asked %v", program, got)
		}
	}
}

func TestASnapshotThatChoosesItsSizeOrItsRegionIsAskedAsWritten(t *testing.T) {
	for _, arguments := range [][]string{
		{"snapshot", "--app", "Finder", "-i"},
		{"snapshot", "--app", "Finder", "--skeleton"},
		{"snapshot", "--app", "Finder", "--max-depth=50"},
		{"snapshot", "--root", "@s8f3k2p9:e3"},
		{"find", "--app", "Finder", "--role", "button"},
	} {
		if got := argumentsOf(t, KindRead, "", arguments...); !slices.Equal(got, arguments) {
			t.Errorf("%v was asked as %v", arguments, got)
		}
	}
	if got := argumentsOf(t, KindRead, "agent-browser", "snapshot", "-s", "#main"); !slices.Equal(got, []string{"snapshot", "-s", "#main", "--json", "--headed"}) {
		t.Errorf("a region of a page was asked as %v", got)
	}
}

func TestAShallowerSnapshotKeepsWhatIsObservedAndReplacesHowMuchOfIt(t *testing.T) {
	for _, asked := range [][]string{
		{"snapshot", "--app", "Finder", "--max-depth", "20", "--interactive-only"},
		{"snapshot", "--app", "Finder", "--max-depth=20", "--skeleton", "-i", "--compact"},
		{"snapshot", "--app", "Finder"},
	} {
		call, _ := NewCall(KindRead, "", asked, "")
		want := []string{"snapshot", "--app", "Finder", "--skeleton", "-i", "--compact", "--max-depth", "2"}
		if got := call.Shallower(2).Arguments; !slices.Equal(got, want) {
			t.Errorf("%v became %v", asked, got)
		}
	}
	page, _ := NewCall(KindRead, "agent-browser", []string{"snapshot", "-d", "9"}, "")
	if got := page.Shallower(3).Arguments; !slices.Equal(got, []string{"snapshot", "--json", "--headed", "-i", "-c", "-d", "3"}) {
		t.Errorf("a page became %v", got)
	}
}

func TestAnAnswerIsMeasuredInCharactersAgainstOneLimit(t *testing.T) {
	if IsTooLong(strings.Repeat("가", LongestObservation)) || !IsTooLong(strings.Repeat("가", LongestObservation+1)) {
		t.Fatal("the limit is not counted in characters")
	}
}
