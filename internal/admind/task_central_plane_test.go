package admind

import (
	"reflect"
	"testing"
)

func TestAQuickTaskAsksTaskAddOnlyForWhatItKnows(t *testing.T) {
	input := taskAddInputOf(Task{Content: "분기 보고서", Status: taskStatusRequested, Size: "M"}, "초안까지", []string{"sample@example.com"})

	want := map[string]any{
		"title":                  "분기 보고서",
		"note":                   "초안까지",
		"size":                   "M",
		"participantPersonHints": []string{"sample@example.com"},
	}
	if !reflect.DeepEqual(input, want) {
		t.Fatalf("input = %#v, want %#v; the record decides a request and the labels left out", input, want)
	}
}

func TestAQuickTaskKeepsAStatusTheNoteGave(t *testing.T) {
	if status := statusTaskAddTakes(taskStatusCompleted); status != taskStatusCompleted {
		t.Fatalf("status = %q", status)
	}
}
