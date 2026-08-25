package admind

import (
	"regexp"
	"testing"
)

// The words a status is called are shared between this service and the company
// app, which is written in another language. The app's own list is the one that
// decides, so this reads it rather than keeping a second copy in agreement by
// hand.
func TestTheStatusWordsAgreeWithTheCompanyApp(t *testing.T) {
	statusWords := readCompanyStatusWords(t)
	central := readCompanyCentralStatusPairs(t, statusWords)

	if len(central) != len(centralStatusOfDeviceStatus) {
		t.Fatalf("the app names %d statuses and this service names %d", len(central), len(centralStatusOfDeviceStatus))
	}
	for centralStatus, word := range central {
		if centralStatusOfDeviceStatus[word] != centralStatus {
			t.Fatalf("the app calls %q %q, and this service calls it %q", centralStatus, word, centralStatusOfDeviceStatus[word])
		}
	}
}

func readCompanyStatusWords(t *testing.T) map[string]string {
	t.Helper()
	source := readRepositoryFile(t, "web", "src", "routes", "flow", "flow-status.ts")
	words := map[string]string{}
	for _, match := range regexp.MustCompile(`(?m)^\t(\w+): '([^']+)'`).FindAllStringSubmatch(source, -1) {
		words[match[1]] = match[2]
	}
	if len(words) == 0 {
		t.Fatal("the company app names no statuses, so this test is reading the wrong file")
	}
	return words
}

func readCompanyCentralStatusPairs(t *testing.T, statusWords map[string]string) map[string]string {
	t.Helper()
	source := readRepositoryFile(t, "web", "src", "lib", "flow", "central-flow-task.ts")
	pairs := map[string]string{}
	for _, match := range regexp.MustCompile(`\['(\w+)', flowStatus\.(\w+)\]`).FindAllStringSubmatch(source, -1) {
		word, named := statusWords[match[2]]
		if !named {
			t.Fatalf("the app pairs %q with %q, which it never names", match[1], match[2])
		}
		pairs[match[1]] = word
	}
	if len(pairs) == 0 {
		t.Fatal("the company app pairs no statuses, so this test is reading the wrong file")
	}
	return pairs
}
