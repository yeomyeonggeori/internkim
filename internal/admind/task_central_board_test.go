package admind

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/centralplane"
)

func TestTheBoardNamesTheSameWorkRuleAsTheAppAndTheRecord(t *testing.T) {
	app := readRepositoryFile(t, "web", "src", "lib", "server", "public-api", "record", "tasks.ts")
	named := regexp.MustCompile(`export const workColumn = '([a-z_]+)';`).FindStringSubmatch(app)
	if named == nil {
		t.Fatal("the app names no work column, so this test is reading the wrong file")
	}
	if named[1] != centralplane.WorkColumn {
		t.Fatalf("the app reads work through %q and the device through %q", named[1], centralplane.WorkColumn)
	}
	if !recordDefinesFunction(t, centralplane.WorkColumn) {
		t.Fatalf("no migration defines public.%s(task public.task)", centralplane.WorkColumn)
	}
}

func TestTheBoardReadsTheLastDayTheRecordAnswers(t *testing.T) {
	if !recordDefinesFunction(t, "task_last_day") {
		t.Fatal("no migration defines public.task_last_day(task public.task)")
	}
	converted := taskOfBoardTask(centralplane.BoardTask{EndsAt: "2026-10-12T15:00:00+00:00", LastDay: "2026-10-12"}, nil, nil)
	if converted.EndDate != "2026-10-12" {
		t.Fatalf("the board ended a task on %s, not the day the record answered", converted.EndDate)
	}
}

func recordDefinesFunction(t *testing.T, name string) bool {
	t.Helper()
	migrations := readRepositoryDirectory(t, "supabase", "migrations")
	definition := regexp.MustCompile(`create (or replace )?function public\.` + name + `\(task public\.task\)`)
	for _, migration := range migrations {
		if definition.MatchString(migration) {
			return true
		}
	}
	return false
}

func readRepositoryDirectory(t *testing.T, pathParts ...string) []string {
	t.Helper()
	directory := filepath.Join(append([]string{"..", ".."}, pathParts...)...)
	entries, errorValue := os.ReadDir(directory)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	documents := make([]string, 0, len(entries))
	for _, entry := range entries {
		document, errorValue := os.ReadFile(filepath.Join(directory, entry.Name()))
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		documents = append(documents, string(document))
	}
	return documents
}
