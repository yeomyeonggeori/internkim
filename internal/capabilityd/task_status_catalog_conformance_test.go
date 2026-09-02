package capabilityd

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
)

func statusesTheCentralPlaneDeclares(t *testing.T) []string {
	t.Helper()
	source, errorValue := os.ReadFile(filepath.Join("..", "..", "supabase", "functions", "_shared", "central-task-status.ts"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	declaration := regexp.MustCompile(`centralTaskStatuses = \[([^\]]+)\]`).FindStringSubmatch(string(source))
	if declaration == nil {
		t.Fatal("the central plane no longer declares centralTaskStatuses as an array, so this test is reading the wrong file")
	}
	statuses := []string{}
	for _, match := range regexp.MustCompile(`'([a-z_]+)'`).FindAllStringSubmatch(declaration[1], -1) {
		statuses = append(statuses, match[1])
	}
	if len(statuses) == 0 {
		t.Fatal("centralTaskStatuses names no statuses")
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

func TestTheTaskSkillTellsTheModelTheStatusesTheCentralPlaneDeclares(t *testing.T) {
	declared := statusesTheCentralPlaneDeclares(t)
	skill, errorValue := os.ReadFile(filepath.Join("..", "..", ".dependency", "internkim-plugin", "skills", "internkim-task", "SKILL.md"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	english := namesTheSkillLists(t, string(skill), `canonical English \(([^)]+)\)`)
	korean := namesTheSkillLists(t, string(skill), `Korean: ([^)]+)\)`)

	slices.Sort(english)
	if !slices.Equal(english, declared) {
		t.Fatalf("the skill tells the model %v and the central plane declares %v", english, declared)
	}
	if len(korean) != len(declared) {
		t.Fatalf("the skill gives %d Korean words for %d statuses", len(korean), len(declared))
	}
}

func namesTheSkillLists(t *testing.T, skill string, pattern string) []string {
	t.Helper()
	match := regexp.MustCompile(pattern).FindStringSubmatch(skill)
	if match == nil {
		t.Fatalf("the skill no longer names its statuses as %q, so this test is reading the wrong sentence", pattern)
	}
	names := strings.Split(match[1], ",")
	for index, name := range names {
		names[index] = strings.TrimSpace(name)
	}
	return names
}
