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
	for _, expectedName := range []string{"preflight", "binaries", "skills", "blueclaw-runtime-base", "blueclaw-config", "blueclaw-payload", "openrouter", "local-llm", "tunnel", "mattermost", "services", "users-sync", "health"} {
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

func TestOnlyCloudflareAccessDoesNotIncludeRuntimeOrTunnelSteps(t *testing.T) {
	context := &Context{
		Backend: BackendSSH,
		SSH:     blueclawPlanBoardConnection{},
		BoardIP: "192.0.2.10",
		Callbacks: Callbacks{
			AdminWebVersion: func() string { return "web-version" },
			LoadState:       func(key string) string { return "web-version" },
		},
	}
	plan, err := DefaultRegistry().resolve(context, Selector{Only: []string{"cloudflare-access"}, Force: true})
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	joinedPlan := strings.Join(plan, ",")
	if joinedPlan != "web,cloudflare-access" {
		t.Fatalf("unexpected plan: %s", joinedPlan)
	}
	for _, disallowedName := range []string{"binaries", "local-llm", "tunnel"} {
		if strings.Contains(joinedPlan, disallowedName) {
			t.Fatalf("cloudflare-access sync must not include %s, got %s", disallowedName, joinedPlan)
		}
	}
}

func TestForcedCloudflareAccessRunsSatisfiedWebDependency(t *testing.T) {
	var webRan bool
	var accessRan bool
	context := &Context{
		Backend: BackendSSH,
		SSH:     blueclawPlanBoardConnection{},
		BoardIP: "192.0.2.10",
		Callbacks: Callbacks{
			AdminWebVersion: func() string { return "web-version" },
			LoadState:       func(key string) string { return "web-version" },
			DeployAdminWeb: func(context *Context) error {
				webRan = true
				return nil
			},
			SyncCloudflareAccess: func(context *Context) error {
				accessRan = true
				return nil
			},
		},
	}

	errorValue := DefaultRegistry().Run(context, Selector{Only: []string{"cloudflare-access"}, Force: true})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !webRan {
		t.Fatalf("forced cloudflare-access must run satisfied web dependency")
	}
	if !accessRan {
		t.Fatalf("cloudflare-access sync did not run")
	}
}

func TestOnlyBlueclawPayloadIncludesStaleBlueclawConfiguration(t *testing.T) {
	context := defaultBlueclawPlanContext("runtime-profile-missing-tools:file.attach", "missing")

	plan, err := DefaultRegistry().resolve(context, Selector{Only: []string{"blueclaw-payload"}})
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}

	joinedPlan := strings.Join(plan, ",")
	if !strings.Contains(joinedPlan, "blueclaw-config") {
		t.Fatalf("expected stale blueclaw configuration to be planned, got %s", joinedPlan)
	}
	if strings.Index(joinedPlan, "blueclaw-config") > strings.Index(joinedPlan, "blueclaw-payload") {
		t.Fatalf("expected blueclaw configuration before payload, got %s", joinedPlan)
	}
}

func TestOnlyServicesIncludesStaleBlueclawConfiguration(t *testing.T) {
	context := defaultBlueclawPlanContext("runtime-profile-missing-tools:file.attach", "ok")

	plan, err := DefaultRegistry().resolve(context, Selector{Only: []string{"services"}})
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}

	joinedPlan := strings.Join(plan, ",")
	if !strings.Contains(joinedPlan, "blueclaw-config") {
		t.Fatalf("expected stale blueclaw configuration to be planned, got %s", joinedPlan)
	}
	if strings.Index(joinedPlan, "blueclaw-config") > strings.Index(joinedPlan, "services") {
		t.Fatalf("expected blueclaw configuration before services, got %s", joinedPlan)
	}
}

func TestOnlyBlueclawPayloadSkipsCurrentBlueclawConfiguration(t *testing.T) {
	context := defaultBlueclawPlanContext("ok", "missing")

	plan, err := DefaultRegistry().resolve(context, Selector{Only: []string{"blueclaw-payload"}})
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}

	joinedPlan := strings.Join(plan, ",")
	if strings.Contains(joinedPlan, "blueclaw-config") {
		t.Fatalf("expected current blueclaw configuration to be skipped, got %s", joinedPlan)
	}
	if !strings.Contains(joinedPlan, "blueclaw-payload") {
		t.Fatalf("expected payload to remain planned, got %s", joinedPlan)
	}
}

func TestOnlyServicesIncludesStaleSkills(t *testing.T) {
	context := defaultBlueclawPlanContext("ok", "ok")
	context.SSH = blueclawPlanBoardConnection{
		runtimeContractOutput: "ok",
		payloadManifestOutput: "ok",
		skillsManifestOutput:  "missing",
	}

	plan, err := DefaultRegistry().resolve(context, Selector{Only: []string{"services"}})
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}

	joinedPlan := strings.Join(plan, ",")
	if !strings.Contains(joinedPlan, "skills") {
		t.Fatalf("expected stale skills to be planned, got %s", joinedPlan)
	}
	if strings.Index(joinedPlan, "skills") > strings.Index(joinedPlan, "services") {
		t.Fatalf("expected skills before services, got %s", joinedPlan)
	}
}

func TestOnlyServicesSkipsCurrentSkills(t *testing.T) {
	context := defaultBlueclawPlanContext("ok", "ok")

	plan, err := DefaultRegistry().resolve(context, Selector{Only: []string{"services"}})
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}

	if strings.Contains(strings.Join(plan, ","), "skills") {
		t.Fatalf("expected current skills to be skipped, got %v", plan)
	}
}

func TestOnlyServicesSimulationSkipsStaleLocalLLM(t *testing.T) {
	context := defaultBlueclawPlanContext("ok", "ok")
	context.BoardType = BoardSimulation

	plan, err := DefaultRegistry().resolve(context, Selector{Only: []string{"services"}})
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}

	joinedPlan := strings.Join(plan, ",")
	if strings.Contains(joinedPlan, "local-llm") {
		t.Fatalf("expected simulation services to skip local-llm, got %s", joinedPlan)
	}
	if !strings.Contains(joinedPlan, "services") {
		t.Fatalf("expected services to remain planned, got %s", joinedPlan)
	}
}

func defaultBlueclawPlanContext(runtimeContractOutput string, payloadManifestOutput string) *Context {
	return &Context{
		Backend: BackendSSH,
		BoardIP: "192.0.2.10",
		SSH: blueclawPlanBoardConnection{
			runtimeContractOutput: runtimeContractOutput,
			payloadManifestOutput: payloadManifestOutput,
			skillsManifestOutput:  "ok",
		},
		Callbacks: Callbacks{
			AdminWebVersion: func() string { return "web-version" },
			SkillsManifest: func() string {
				return "skills-manifest"
			},
			BlueclawPayloadManifest: func() string {
				return "payload-manifest"
			},
			LoadState: func(key string) string {
				if key == "web_version" || key == "admin_web_version" {
					return "web-version"
				}
				return ""
			},
		},
	}
}

type blueclawPlanBoardConnection struct {
	runtimeContractOutput string
	payloadManifestOutput string
	skillsManifestOutput  string
}

func (connection blueclawPlanBoardConnection) Run(command string) string {
	switch {
	case strings.Contains(command, "runtime_path ="):
		return connection.runtimeContractOutput
	case strings.Contains(command, "payload-manifest.json"):
		return connection.payloadManifestOutput
	case strings.Contains(command, ".internkim-skills-manifest.json"):
		return connection.skillsManifestOutput
	case strings.Contains(command, "rootfs_path="):
		return "ok"
	case strings.Contains(command, "blkid -o value -s TYPE"):
		return "ok"
	case strings.Contains(command, "test -e "):
		return "y"
	case strings.Contains(command, "systemctl cat llama"):
		return "/usr/local/bin/llama-server"
	default:
		return "ok"
	}
}

func (connection blueclawPlanBoardConnection) SCP(localPath, remotePath string) error {
	return nil
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
