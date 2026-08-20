package admind

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// docs/internal/task-sync-direction.md §7: retirement repoints one seam, and it
// is only a seam while every read of the device's copy goes through it. The two
// stores are that seam; the sync machinery retires with the copy it syncs.
func TestNothingReadsTheFlowTasksTableBehindTheStores(t *testing.T) {
	allowed := map[string]bool{
		"flow_task_read_store.go":      true,
		"flow_task_write_store.go":     true,
		"flow_central_mirror_mark.go":  true,
		"flow_central_event_repair.go": true,
		"flow_central_backfill.go":     true,
	}

	entries, errorValue := os.ReadDir(".")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") || allowed[name] {
			continue
		}
		document, errorValue := os.ReadFile(filepath.Join(".", name))
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if strings.Contains(string(document), "FROM flow_tasks") {
			t.Errorf("%s reads flow_tasks directly; put the query in flow_task_read_store.go so retirement has one place to repoint", name)
		}
	}
}
