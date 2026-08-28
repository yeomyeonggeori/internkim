package admind

import (
	"regexp"
	"testing"
)

// The status vocabulary is shared with the company app, which is written in
// another language. The app's own list is the one read here, rather than a
// second copy kept in agreement by hand.
func TestTheStatusWordsAgreeWithTheCompanyApp(t *testing.T) {
	source := readRepositoryFile(t, "web", "src", "routes", "task", "task-status.ts")
	appStatuses := map[string]bool{}
	for _, match := range regexp.MustCompile(`(?m)^\t(\w+): '([a-z_]+)'`).FindAllStringSubmatch(source, -1) {
		appStatuses[match[2]] = true
	}
	if len(appStatuses) == 0 {
		t.Fatal("the company app names no statuses, so this test is reading the wrong file")
	}

	if len(appStatuses) != len(taskStatusOptions()) {
		t.Fatalf("the app names %d statuses and this service names %d", len(appStatuses), len(taskStatusOptions()))
	}
	for _, status := range taskStatusOptions() {
		if !appStatuses[status] {
			t.Fatalf("this service names %q, which the app never does", status)
		}
	}
}
