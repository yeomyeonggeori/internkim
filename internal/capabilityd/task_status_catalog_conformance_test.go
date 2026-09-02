package capabilityd

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"
)

func statusesTheCentralPlaneDeclares(t *testing.T) []string {
	t.Helper()
	source, errorValue := os.ReadFile(filepath.Join("..", "..", "supabase", "functions", "_shared", "central-task-status.ts"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	statuses := []string{}
	for _, match := range regexp.MustCompile(`\n\t'([a-z_]+)'`).FindAllStringSubmatch(string(source), -1) {
		statuses = append(statuses, match[1])
	}
	if len(statuses) == 0 {
		t.Fatal("the central plane names no statuses, so this test is reading the wrong file")
	}
	slices.Sort(statuses)
	return statuses
}

func TestTheToolCatalogTakesTheStatusesTheCentralPlaneDeclares(t *testing.T) {
	declared := statusesTheCentralPlaneDeclares(t)

	takenOnUpdate := taskUpdateStatuses()
	slices.Sort(takenOnUpdate)
	if !slices.Equal(takenOnUpdate, declared) {
		t.Fatalf("task_update takes %v and the central plane declares %v", takenOnUpdate, declared)
	}
}

func TestTheToolCatalogOpensATaskOnEveryStatusButRequested(t *testing.T) {
	initial := slices.DeleteFunc(statusesTheCentralPlaneDeclares(t), func(status string) bool {
		return status == "requested"
	})

	takenOnAdd := taskAddStatuses()
	slices.Sort(takenOnAdd)
	if !slices.Equal(takenOnAdd, initial) {
		t.Fatalf("task_add takes %v and a task may open on %v; the runtime, not the model, asks for requested", takenOnAdd, initial)
	}
}
