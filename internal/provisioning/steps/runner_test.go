package setup

import (
	"strings"
	"testing"
)

func TestDefaultPlanMarksSatisfiedStepsSkipped(t *testing.T) {
	registry := testRegistry(false, true, false)
	context := &Context{Backend: BackendSSH}

	entries, err := registry.plan(context, Selector{})
	if err != nil {
		t.Fatalf("plan failed: %v", err)
	}

	statuses := entryStatuses(entries)
	if statuses != "alpha=run,beta=skip:satisfied,gamma=run" {
		t.Fatalf("unexpected statuses: %s", statuses)
	}
}

func TestOnlyIncludesUnsatisfiedDependencies(t *testing.T) {
	registry := testRegistry(false, false, false)
	context := &Context{Backend: BackendSSH}

	plan, err := registry.resolve(context, Selector{Only: []string{"gamma"}})
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}

	if strings.Join(plan, ",") != "alpha,beta,gamma" {
		t.Fatalf("unexpected plan: %v", plan)
	}
}

func TestSkipExcludesRequestedSteps(t *testing.T) {
	registry := testRegistry(false, false, false)
	context := &Context{Backend: BackendSSH}

	plan, err := registry.resolve(context, Selector{Skip: []string{"beta"}})
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}

	if strings.Join(plan, ",") != "alpha,gamma" {
		t.Fatalf("unexpected plan: %v", plan)
	}
}

func TestForceRerunsOnlyExplicitSeedDependenciesStaySatisfied(t *testing.T) {
	registry := testRegistry(false, true, true)
	context := &Context{Backend: BackendSSH}

	plan, err := registry.resolve(context, Selector{Only: []string{"gamma"}, Force: true})
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}

	if strings.Join(plan, ",") != "gamma" {
		t.Fatalf("unexpected plan: %v", plan)
	}
}

func TestDefaultForceTreatsAllDefaultSeedsAsExplicit(t *testing.T) {
	registry := testRegistry(true, true, true)
	context := &Context{Backend: BackendSSH}

	entries, err := registry.plan(context, Selector{Force: true})
	if err != nil {
		t.Fatalf("plan failed: %v", err)
	}

	statuses := entryStatuses(entries)
	if statuses != "alpha=run,beta=run,gamma=run" {
		t.Fatalf("unexpected statuses: %s", statuses)
	}
}

func TestForceAllIncludesSatisfiedDependencies(t *testing.T) {
	registry := testRegistry(true, true, true)
	context := &Context{Backend: BackendSSH}

	plan, err := registry.resolve(context, Selector{Only: []string{"gamma"}, ForceAll: true})
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}

	if strings.Join(plan, ",") != "alpha,beta,gamma" {
		t.Fatalf("unexpected plan: %v", plan)
	}
}

func TestDryRunDoesNotExecuteSteps(t *testing.T) {
	runCount := 0
	registry := Registry{{
		Name: "alpha",
		Run: func(context *Context) error {
			runCount++
			return nil
		},
	}}

	err := registry.Run(&Context{Backend: BackendSSH}, Selector{DryRun: true})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if runCount != 0 {
		t.Fatalf("expected dry run not to execute, got %d runs", runCount)
	}
}

func TestExplicitUnsupportedBackendReturnsError(t *testing.T) {
	registry := Registry{{
		Name: "alpha",
		Run:  func(context *Context) error { return nil },
	}}

	err := registry.Run(&Context{Backend: BackendSD}, Selector{Only: []string{"alpha"}})
	if err == nil || !strings.Contains(err.Error(), "step does not support this backend") {
		t.Fatalf("expected unsupported backend error, got %v", err)
	}
}

func TestJetsonDefaultResolveIncludesLocalLLMAndSkipsGoogle(t *testing.T) {
	context := &Context{Backend: BackendSSH, BoardType: BoardJetsonOrinNano}
	plan, err := DefaultRegistry().resolve(context, Selector{Skip: []string{"google"}})
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}

	joinedPlan := strings.Join(plan, ",")
	for _, expectedName := range []string{"preflight", "binaries", "skills", "blueclaw-runtime-base", "blueclaw-payload", "openrouter", "local-llm", "tunnel", "mattermost", "services", "users-sync", "health"} {
		if !strings.Contains(joinedPlan, expectedName) {
			t.Fatalf("expected plan to include %s, got %s", expectedName, joinedPlan)
		}
	}
	if strings.Contains(joinedPlan, "ollama") {
		t.Fatalf("expected default plan to skip ollama, got %s", joinedPlan)
	}
	if strings.Contains(joinedPlan, "google") {
		t.Fatalf("expected plan to skip google, got %s", joinedPlan)
	}
}

func TestAdminWebAliasResolvesToWeb(t *testing.T) {
	names := ParseNames("admin-web,binaries")
	if strings.Join(names, ",") != "web,binaries" {
		t.Fatalf("expected admin-web alias to become web, got %v", names)
	}

	registry := Registry{
		{Name: "web", Run: func(context *Context) error { return nil }},
		{Name: "binaries", Deps: []string{"web"}, Run: func(context *Context) error { return nil }},
	}
	plan, err := registry.resolve(&Context{Backend: BackendSSH}, Selector{Only: []string{"admin-web"}})
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if strings.Join(plan, ",") != "web" {
		t.Fatalf("unexpected plan: %v", plan)
	}
}

func testRegistry(alphaSatisfied bool, betaSatisfied bool, gammaSatisfied bool) Registry {
	return Registry{
		{
			Name: "alpha",
			IsSatisfied: func(context *Context) bool {
				return alphaSatisfied
			},
			Run: func(context *Context) error { return nil },
		},
		{
			Name: "beta",
			Deps: []string{"alpha"},
			IsSatisfied: func(context *Context) bool {
				return betaSatisfied
			},
			Run: func(context *Context) error { return nil },
		},
		{
			Name: "gamma",
			Deps: []string{"beta"},
			IsSatisfied: func(context *Context) bool {
				return gammaSatisfied
			},
			Run: func(context *Context) error { return nil },
		},
	}
}

func entryStatuses(entries []planEntry) string {
	var statuses []string
	for _, entry := range entries {
		status := entry.name + "=" + entry.status
		if entry.reason != "" {
			status += ":" + entry.reason
		}
		statuses = append(statuses, status)
	}
	return strings.Join(statuses, ",")
}
