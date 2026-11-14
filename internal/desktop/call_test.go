package desktop

import (
	"slices"
	"strings"
	"testing"
)

func TestAReadRefusesACommandThatChangesTheDesktopAndAnActRunsEither(t *testing.T) {
	if _, errorValue := NewCall(KindRead, "", []string{"snapshot", "--app", "Finder", "-i"}, ""); errorValue != nil {
		t.Fatal(errorValue)
	}
	_, errorValue := NewCall(KindRead, "", []string{"click", "@s1:e2"}, "")
	if errorValue == nil || !strings.Contains(errorValue.Error(), "desktop_act") {
		t.Fatalf("a read that clicks was answered %v", errorValue)
	}
	for _, arguments := range [][]string{{"snapshot", "-i"}, {"list-windows"}, {"click", "@s1:e2"}} {
		call, errorValue := NewCall(KindAct, "", arguments, "")
		if errorValue != nil || !slices.Equal(call.Arguments, arguments) {
			t.Errorf("an act of %v was built %+v, %v", arguments, call, errorValue)
		}
	}
}

func TestAReadNamesOnlyTheCommandsThatChangeNothing(t *testing.T) {
	_, errorValue := NewCall(KindRead, "", []string{"session"}, "")
	if errorValue == nil || !strings.Contains(errorValue.Error(), "snapshot") || strings.Contains(errorValue.Error(), "click") {
		t.Fatalf("an unknown read was answered %v", errorValue)
	}
}

func TestACommandOutsideBothListsIsRefusedWithTheListThatIsRun(t *testing.T) {
	for _, command := range []string{"mcp", "batch", "session", "--headed", ""} {
		_, errorValue := NewCall(KindAct, "", []string{command, "x"}, "")
		if errorValue == nil || !strings.Contains(errorValue.Error(), "click") {
			t.Errorf("%q was answered %v", command, errorValue)
		}
	}
	if _, errorValue := NewCall(KindRead, "", nil, ""); errorValue == nil {
		t.Error("a call with no arguments was built")
	}
}

func TestTextTravelsOnStandardInputAndNeverAsAnArgument(t *testing.T) {
	arguments := []string{"type", "@s1:e2"}
	call, errorValue := NewCall(KindAct, "", arguments, "분기 보고서")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !slices.Equal(call.Arguments, []string{"type", "@s1:e2", "--stdin"}) || call.Text != "분기 보고서" {
		t.Fatalf("built %+v", call)
	}
	if len(arguments) != 2 {
		t.Fatal("the caller's arguments were changed")
	}
}

func TestNoCommandIsBothAReadAndAnAct(t *testing.T) {
	for name, program := range Programs {
		for _, command := range program.Read {
			if program.Acts(command) {
				t.Errorf("%s %q is listed as both", name, command)
			}
		}
	}
}

func TestAScreenshotIsLiftedOutOfTheEnvelope(t *testing.T) {
	output := `{"version":"2.6","ok":true,"command":"screenshot","data":{"data":"aGVsbG8=","format":"png","height":2,"width":3}}`
	screenshot, isScreenshot := ScreenshotOf(output)
	if !isScreenshot || screenshot.PNGBase64 != "aGVsbG8=" {
		t.Fatalf("lifted %+v (%v)", screenshot, isScreenshot)
	}
	if strings.Contains(screenshot.Remainder, "aGVsbG8=") || !strings.Contains(screenshot.Remainder, `"width":3`) {
		t.Fatalf("the remainder is %s", screenshot.Remainder)
	}
	if _, isScreenshot := ScreenshotOf(`{"ok":true,"command":"snapshot","data":{"tree":{}}}`); isScreenshot {
		t.Fatal("a snapshot was read as a screenshot")
	}
}

func TestTheEnvelopeIsTheLastLineSoANoticeBeforeItIsNotReadAsOne(t *testing.T) {
	envelope, errorValue := ReadEnvelope("a notice on the first line\n" + `{"ok":false,"command":"click","error":{"code":"STALE_REF"}}`)
	if errorValue != nil || envelope.Succeeded() {
		t.Fatalf("read %+v, %v", envelope, errorValue)
	}
	if _, errorValue := ReadEnvelope("not json"); errorValue == nil {
		t.Fatal("prose was read as an envelope")
	}
}

func TestABrowserCallAsksForJSONInAVisibleWindowAndCarriesItsTextLast(t *testing.T) {
	call, errorValue := NewCall(KindAct, "agent-browser", []string{"fill", "@e3"}, "분기 보고서")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if call.Program != "agent-browser" || !slices.Equal(call.Arguments, []string{"fill", "@e3", "--json", "--headed", "분기 보고서"}) || call.Text != "" {
		t.Fatalf("built %+v", call)
	}
}

func TestAProgramOutsideTheSetIsRefusedByName(t *testing.T) {
	_, errorValue := NewCall(KindAct, "osascript", []string{"click"}, "")
	if errorValue == nil || !strings.Contains(errorValue.Error(), "agent-browser, agent-desktop") {
		t.Fatalf("answered %v", errorValue)
	}
}

func TestACallWithNoProgramRunsAgentDesktop(t *testing.T) {
	call, errorValue := NewCall(KindRead, "", []string{"snapshot"}, "")
	if errorValue != nil || call.Program != DefaultProgram {
		t.Fatalf("built %+v, %v", call, errorValue)
	}
}

func TestBothEnvelopesSayWhetherTheCommandSucceeded(t *testing.T) {
	for output, succeeded := range map[string]bool{
		`{"ok":true,"command":"click"}`:             true,
		`{"ok":false,"command":"click"}`:            false,
		`{"success":true,"data":{},"error":null}`:   true,
		`{"success":false,"data":null,"error":"x"}`: false,
	} {
		envelope, errorValue := ReadEnvelope(output)
		if errorValue != nil || envelope.Succeeded() != succeeded {
			t.Errorf("%s was read as %+v, %v", output, envelope, errorValue)
		}
	}
}
