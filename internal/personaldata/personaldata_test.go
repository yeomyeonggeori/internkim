package personaldata

import (
	"io"
	"regexp"
	"strings"
	"testing"
)

var anyToken = regexp.MustCompile(`\{\{[A-Za-z]+:[0-9a-f]+\}\}`)

func maskedOnce(t *testing.T, masker Masker, document string) (string, *Substitutions) {
	t.Helper()
	substitutions := NewSubstitutions()
	masked, isJSON := masker.MaskJSON([]byte(document), substitutions)
	if !isJSON {
		t.Fatalf("a JSON body must be read as JSON: %s", document)
	}
	return string(masked), substitutions
}

func TestAPhoneNumberLeavesAsATokenAndTheTokenComesBackAsTheNumber(t *testing.T) {
	masked, substitutions := maskedOnce(t, NewMasker(), `{"messages":[{"role":"user","content":"제 번호는 010-1234-5678이에요"}]}`)
	if strings.Contains(masked, "1234-5678") {
		t.Fatalf("the number must not reach the model, sent %s", masked)
	}
	token := anyToken.FindString(masked)
	if !strings.HasPrefix(token, "{{phoneNumber:") || !strings.Contains(masked, token+"이에요") {
		t.Fatalf("the number must become one phone number token in place, sent %s", masked)
	}
	answer := `{"choices":[{"message":{"content":"","tool_calls":[{"function":{"arguments":"{\"to\":\"` + token + `\"}"}}]}}]}`
	restored := string(substitutions.UnmaskJSON([]byte(answer)))
	if !strings.Contains(restored, `{\"to\":\"010-1234-5678\"}`) {
		t.Fatalf("a tool call naming the token must carry the number, got %s", restored)
	}
}

func TestOneValueIsOneTokenAcrossCallsAndEveryWayOfWritingIt(t *testing.T) {
	masker := NewMasker()
	first, _ := maskedOnce(t, masker, `["010-1234-5678"]`)
	second, _ := maskedOnce(t, masker, `["01012345678 다시 보내요"]`)
	if anyToken.FindString(first) != anyToken.FindString(second) {
		t.Fatalf("the same number must be the same token so the provider's cache and the model's reading hold, got %s and %s", first, second)
	}
}

func TestATokenTheModelNeverSawStaysAsWritten(t *testing.T) {
	_, substitutions := maskedOnce(t, NewMasker(), `["010-1234-5678"]`)
	invented := `["{{phoneNumber:0123abcd}}"]`
	if restored := string(substitutions.UnmaskJSON([]byte(invented))); restored != invented {
		t.Fatalf("an invented token must not become anyone's number, got %s", restored)
	}
}

func TestKeysPicturesAndUntouchedBytesStayAsSent(t *testing.T) {
	document := `{ "010-1234-5678" : "data:image/png;base64,01012345678",  "b":1, "a":"no numbers" }`
	masked, substitutions := maskedOnce(t, NewMasker(), document)
	if masked != document || !substitutions.IsEmpty() {
		t.Fatalf("keys, pictures, order and spacing must reach the provider unchanged, got %s", masked)
	}
}

func TestOnlyNumbersThatCanBeRealAreMasked(t *testing.T) {
	cases := []struct {
		text     string
		isMasked bool
	}{
		{"900101-1234567", true},
		{"9001011234568", true},
		{"9001011234567", false},
		{"4111 1111 1111 1111", true},
		{"4111 1111 1111 1112", false},
		{"02-123-4567", true},
		{"+82 10-1234-5678", true},
		{"주문번호 2026-1127-0001", false},
		{"20261127123456", false},
		{"버전 1.10.1234.5678", false},
	}
	masker := NewMasker()
	for _, testCase := range cases {
		substitutions := NewSubstitutions()
		masked := masker.MaskText(testCase.text, substitutions)
		if isMasked := masked != testCase.text; isMasked != testCase.isMasked {
			t.Errorf("%q masked=%v, want %v (sent %q)", testCase.text, isMasked, testCase.isMasked, masked)
		}
	}
}

func streamedAnswer(t *testing.T, substitutions *Substitutions, events ...string) string {
	t.Helper()
	var body strings.Builder
	for _, event := range events {
		body.WriteString("data: " + event + "\n\n")
	}
	restored, errorValue := io.ReadAll(substitutions.UnmaskingEventStream(io.NopCloser(strings.NewReader(body.String()))))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return string(restored)
}

func contentOf(t *testing.T, stream string, field *regexp.Regexp) string {
	t.Helper()
	var joined strings.Builder
	for _, match := range field.FindAllStringSubmatch(stream, -1) {
		joined.WriteString(match[1])
	}
	return joined.String()
}

var streamedContent = regexp.MustCompile(`"content":"([^"]*)"`)
var streamedArguments = regexp.MustCompile(`"arguments":"((?:[^"\\]|\\.)*)"`)

func TestATokenSplitAcrossStreamedEventsArrivesWhole(t *testing.T) {
	masked, substitutions := maskedOnce(t, NewMasker(), `["010-1234-5678"]`)
	token := anyToken.FindString(masked)
	stream := streamedAnswer(t, substitutions,
		`{"id":"gen-1","choices":[{"index":0,"delta":{"content":"번호는 `+token[:5]+`"}}]}`,
		`{"id":"gen-1","choices":[{"index":0,"delta":{"content":"`+token[5:12]+`"}}]}`,
		`{"id":"gen-1","choices":[{"index":0,"delta":{"content":"`+token[12:]+` 입니다"}}]}`,
		`{"id":"gen-1","choices":[{"index":0,"delta":{},"finish_reason":"stop"}]}`,
		"[DONE]",
	)
	if content := contentOf(t, stream, streamedContent); content != "번호는 010-1234-5678 입니다" {
		t.Fatalf("the streamed reply must read with the number, got %q from %s", content, stream)
	}
}

func TestToolCallArgumentsSplitAcrossEventsCarryTheValue(t *testing.T) {
	masked, substitutions := maskedOnce(t, NewMasker(), `["900101-1234567"]`)
	token := anyToken.FindString(masked)
	stream := streamedAnswer(t, substitutions,
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call-1","function":{"name":"shell","arguments":"{"}}]}}]}`,
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"\"text\":\"`+token[:3]+`"}}]}}]}`,
		`{"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"`+token[3:]+`\"}"}}]}}]}`,
		`{"choices":[{"index":0,"delta":{},"finish_reason":"tool_calls"}]}`,
	)
	if arguments := contentOf(t, stream, streamedArguments); arguments != `{\"text\":\"900101-1234567\"}` {
		t.Fatalf("the tool must be called with the number, got %q from %s", arguments, stream)
	}
}

func TestTextHeldForATokenIsDeliveredWhenTheStreamEnds(t *testing.T) {
	_, substitutions := maskedOnce(t, NewMasker(), `["010-1234-5678"]`)
	stream := streamedAnswer(t, substitutions,
		`{"id":"gen-2","choices":[{"index":0,"delta":{"content":"끝에 괄호 {"}}]}`,
		"[DONE]",
	)
	if content := contentOf(t, stream, streamedContent); content != "끝에 괄호 {" || !strings.HasSuffix(stream, "data: [DONE]\n\n") {
		t.Fatalf("text held back must reach the agent before the stream says it is done, got %s", stream)
	}
}

func TestEventsThatAreNotJSONPassAsSent(t *testing.T) {
	_, substitutions := maskedOnce(t, NewMasker(), `["010-1234-5678"]`)
	body := ": OPENROUTER PROCESSING\n\ndata: first-from-openrouter\n\n"
	restored, _ := io.ReadAll(substitutions.UnmaskingEventStream(io.NopCloser(strings.NewReader(body))))
	if string(restored) != body {
		t.Fatalf("comments and non-JSON events must pass unchanged, got %q", restored)
	}
}
