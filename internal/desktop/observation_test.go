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

const launchedEnvelope = `{"version":"2.6","ok":true,"command":"launch","data":{"app":"TextEdit","pid":53908,"window":{"accessible":true,"app_name":"TextEdit","id":"w-14363","title":"Open","visible":true}}}`

func observationsAfter(t *testing.T, program string, output string, scopeOfSnapshot func(string) Scope, arguments ...string) [][]string {
	t.Helper()
	act, errorValue := NewCall(KindAct, program, arguments, "")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	envelope, errorValue := ReadEnvelope(output)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	asked := [][]string{}
	for _, observation := range act.ObservationsAfter(envelope, scopeOfSnapshot) {
		asked = append(asked, observation.Arguments)
	}
	return asked
}

func noSnapshotIsRemembered(string) Scope { return Scope{} }

func sameCalls(got [][]string, want [][]string) bool {
	return slices.EqualFunc(got, want, func(first []string, second []string) bool { return slices.Equal(first, second) })
}

const clickedEnvelope = `{"version":"2.6","ok":true,"command":"click","data":{"action":"click","disposition":{"delivery":"delivered_unverified","retry":"unsafe"},"steps":[]}}`

func TestTheScreenAfterAnActIsTheWindowThenTheAppItsEnvelopeOrItsOwnArgumentsName(t *testing.T) {
	for name, example := range map[string]struct {
		output    string
		arguments []string
		want      [][]string
	}{
		"the window a launch reports, then its app": {launchedEnvelope, []string{"launch", "TextEdit"}, [][]string{
			{"snapshot", "--window-id", "w-14363", "-i", "--compact"},
			{"snapshot", "--app", "TextEdit", "-i", "--compact"},
		}},
		"the app a key press was sent to": {clickedEnvelope, []string{"press", "cmd+n", "--app", "TextEdit"}, [][]string{
			{"snapshot", "--app", "TextEdit", "-i", "--compact"},
		}},
		"the window a window command names, then the app it names": {clickedEnvelope, []string{"focus-window", "--window-id=w-7", "--app", "Notes"}, [][]string{
			{"snapshot", "--window-id", "w-7", "-i", "--compact"},
			{"snapshot", "--app", "Notes", "-i", "--compact"},
		}},
		"the app an envelope alone reports": {`{"ok":true,"command":"close-app","data":{"action":"close-app","app":"Calculator"}}`, []string{"close-app", "Calculator"}, [][]string{
			{"snapshot", "--app", "Calculator", "-i", "--compact"},
		}},
	} {
		if got := observationsAfter(t, "", example.output, noSnapshotIsRemembered, example.arguments...); !sameCalls(got, example.want) {
			t.Errorf("%s: asked %v", name, got)
		}
	}
}

func TestTheScreenAfterARefActIsTheWindowItsSnapshotWasTakenOf(t *testing.T) {
	remembered := func(snapshotID string) Scope {
		if snapshotID == "s8f3k2p9" {
			return Scope{Window: "w-99", Application: "Ghostty"}
		}
		return Scope{}
	}
	want := [][]string{{"snapshot", "--window-id", "w-99", "-i", "--compact"}, {"snapshot", "--app", "Ghostty", "-i", "--compact"}}
	if got := observationsAfter(t, "", clickedEnvelope, remembered, "click", "@s8f3k2p9:e4"); !sameCalls(got, want) {
		t.Fatalf("asked %v", got)
	}
	if got := observationsAfter(t, "", clickedEnvelope, remembered, "click", "@sunknown:e4"); len(got) != 0 {
		t.Fatalf("a ref from a snapshot nobody saw was observed with %v", got)
	}
	if got := observationsAfter(t, "", clickedEnvelope, remembered, "mouse-click", "--xy", "10,10"); len(got) != 0 {
		t.Fatalf("a point on the screen was observed with %v", got)
	}
}

func TestTheScreenAfterABrowserActIsThePageWithNoWindowToName(t *testing.T) {
	got := observationsAfter(t, "agent-browser", `{"success":true,"data":{"url":"https://example.com"},"error":null}`, noSnapshotIsRemembered, "open", "https://example.com")
	if !sameCalls(got, [][]string{{"snapshot", "-i", "--json", "--headed"}}) {
		t.Fatalf("asked %v", got)
	}
}

func TestAnEnvelopeWhoseDataIsNotAnObjectIsStillAnEnvelope(t *testing.T) {
	for _, output := range []string{
		`{"version":"2.6","ok":true,"command":"list-windows","data":[{"id":"w-1","app_name":"Finder"}]}`,
		`{"success":true,"data":"about:blank","error":null}`,
		`{"success":true,"data":null,"error":null}`,
		`{"ok":true,"command":"snapshot","data":{"app":"Ghostty","window":"none"}}`,
	} {
		envelope, errorValue := ReadEnvelope(output)
		if errorValue != nil || !envelope.Succeeded() {
			t.Errorf("%s was read %+v, %v", output, envelope, errorValue)
		}
	}
}

func TestAnActIsDeliveredUnlessItsEnvelopeSaysNothingWas(t *testing.T) {
	for output, want := range map[string]bool{
		launchedEnvelope: true,
		`{"ok":true,"command":"check","data":{"disposition":{"delivery":"not_delivered","retry":"safe"}}}`:        false,
		`{"ok":true,"command":"check","data":{"disposition":{"delivery":"delivered_verified","retry":"unsafe"}}}`: true,
		`{"ok":false,"command":"click","error":{"code":"STALE_REF"}}`:                                             false,
		`{"success":true,"data":{"clicked":true},"error":null}`:                                                   true,
	} {
		envelope, _ := ReadEnvelope(output)
		if envelope.Delivered() != want {
			t.Errorf("%s was read as delivered=%v", output, envelope.Delivered())
		}
	}
}
