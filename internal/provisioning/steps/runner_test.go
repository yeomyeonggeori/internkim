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

func TestOnlyPlanDoesNotProbeUnplannedStepSatisfaction(t *testing.T) {
	probed := false
	registry := Registry{
		{
			Name: "alpha",
			Run:  func(context *Context) error { return nil },
		},
		{
			Name: "beta",
			IsSatisfied: func(context *Context) bool {
				probed = true
				return true
			},
			Run: func(context *Context) error { return nil },
		},
	}

	entries, err := registry.plan(&Context{Backend: BackendSSH}, Selector{Only: []string{"alpha"}})
	if err != nil {
		t.Fatalf("plan failed: %v", err)
	}
	if probed {
		t.Fatal("unplanned step satisfaction should not be probed for --only plan")
	}
	if entryStatuses(entries) != "alpha=run" {
		t.Fatalf("unexpected entries: %s", entryStatuses(entries))
	}
}

func TestDryRunPlanDoesNotProbeDependencySatisfaction(t *testing.T) {
	probed := false
	registry := Registry{
		{
			Name: "alpha",
			IsSatisfied: func(context *Context) bool {
				probed = true
				return true
			},
			Run: func(context *Context) error { return nil },
		},
		{
			Name: "beta",
			Deps: []string{"alpha"},
			Run:  func(context *Context) error { return nil },
		},
	}

	entries, err := registry.plan(&Context{Backend: BackendSSH}, Selector{Only: []string{"beta"}, DryRun: true})
	if err != nil {
		t.Fatalf("plan failed: %v", err)
	}
	if probed {
		t.Fatal("dependency satisfaction should not be probed for dry-run plan")
	}
	if entryStatuses(entries) != "alpha=run,beta=run" {
		t.Fatalf("unexpected entries: %s", entryStatuses(entries))
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

func TestRunAcquiresAndReleasesRemoteSetupLock(t *testing.T) {
	connection := &setupLockBoardConnection{}
	runCount := 0
	registry := Registry{{
		Name: "alpha",
		Run: func(context *Context) error {
			runCount++
			return nil
		},
	}}

	err := registry.Run(&Context{
		Backend:      BackendSSH,
		SSH:          connection,
		SetupLockID:  "lock-1",
		SetupCommand: "internkim setup --only alpha",
		SetupSteps:   "--only=alpha",
	}, Selector{})
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}
	if runCount != 1 {
		t.Fatalf("expected step to run once, got %d", runCount)
	}
	commands := strings.Join(connection.commands, "\n---\n")
	for _, expectedText := range []string{"internkim-setup-lock-acquired", `"lockID":"lock-1"`, "rm -rf \"$lock_directory\""} {
		if !strings.Contains(commands, expectedText) {
			t.Fatalf("expected commands to contain %q, got\n%s", expectedText, commands)
		}
	}
}

func TestRunStopsWhenRemoteSetupLockIsBusy(t *testing.T) {
	connection := &setupLockBoardConnection{busy: true}
	registry := Registry{{
		Name: "alpha",
		Run:  func(context *Context) error { return nil },
	}}

	err := registry.Run(&Context{Backend: BackendSSH, SSH: connection, SetupLockID: "lock-1"}, Selector{})
	if err == nil || !strings.Contains(err.Error(), "remote setup is already running") {
		t.Fatalf("expected busy lock error, got %v", err)
	}
	if len(connection.commands) != 1 {
		t.Fatalf("expected only acquire command, got %+v", connection.commands)
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

type setupLockBoardConnection struct {
	commands []string
	busy     bool
}

func (connection *setupLockBoardConnection) Run(command string) string {
	connection.commands = append(connection.commands, command)
	if strings.Contains(command, "mkdir \"$lock_directory\"") {
		if connection.busy {
			return "internkim-setup-lock-busy\n{\"command\":\"other setup\"}"
		}
		return "internkim-setup-lock-acquired"
	}
	return ""
}

func (connection *setupLockBoardConnection) SCP(localPath, remotePath string) error {
	return nil
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

func TestSkillsRunAfterBlueclawRuntimeBase(t *testing.T) {
	context := &Context{Backend: BackendSSH}
	plan, err := DefaultRegistry().resolve(context, Selector{Skip: []string{"google"}})
	if err != nil {
		t.Fatal(err)
	}
	runtimeBaseIndex := setupPlanIndex(plan, "blueclaw-runtime-base")
	skillsIndex := setupPlanIndex(plan, "skills")
	if runtimeBaseIndex < 0 || skillsIndex < 0 || runtimeBaseIndex > skillsIndex {
		t.Fatalf("expected blueclaw-runtime-base before skills, got %s", strings.Join(plan, ","))
	}
}

func TestAdminWebAliasIsNotSupported(t *testing.T) {
	names := ParseNames("admin-web,binaries")
	if strings.Join(names, ",") != "admin-web,binaries" {
		t.Fatalf("unexpected names: %v", names)
	}

	registry := Registry{
		{Name: "web", Run: func(context *Context) error { return nil }},
		{Name: "binaries", Deps: []string{"web"}, Run: func(context *Context) error { return nil }},
	}
	if _, errorValue := registry.resolve(&Context{Backend: BackendSSH}, Selector{Only: []string{"admin-web"}}); errorValue == nil {
		t.Fatal("expected admin-web to be rejected")
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
	for _, disallowedName := range []string{"preflight", "board", "binaries", "local-llm", "tunnel"} {
		if strings.Contains(joinedPlan, disallowedName) {
			t.Fatalf("cloudflare-access sync must not include %s, got %s", disallowedName, joinedPlan)
		}
	}
}

func TestOnlyWebDoesNotRequireBoard(t *testing.T) {
	context := &Context{Backend: BackendSSH}
	plan, err := DefaultRegistry().resolve(context, Selector{Only: []string{"web"}, Force: true})
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	joinedPlan := strings.Join(plan, ",")
	if joinedPlan != "web" {
		t.Fatalf("web deploy should not require board setup, got %s", joinedPlan)
	}
}

func TestOnlyAdmindDoesNotIncludeBroadRuntimeSteps(t *testing.T) {
	context := &Context{
		Backend: BackendSSH,
		SSH:     blueclawPlanBoardConnection{},
		BoardIP: "192.0.2.10",
	}
	plan, err := DefaultRegistry().resolve(context, Selector{Only: []string{"admind"}, Force: true})
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	joinedPlan := strings.Join(plan, ",")
	if joinedPlan != "admind" {
		t.Fatalf("unexpected plan: %s", joinedPlan)
	}
	for _, disallowedName := range []string{"binaries", "services", "local-llm"} {
		if strings.Contains(joinedPlan, disallowedName) {
			t.Fatalf("admind deploy must not include %s, got %s", disallowedName, joinedPlan)
		}
	}
}

func TestOnlyBlueclawPayloadDirectDoesNotIncludeBroadRuntimeSteps(t *testing.T) {
	context := &Context{
		Backend: BackendSSH,
		SSH:     blueclawPlanBoardConnection{},
		BoardIP: "192.0.2.10",
	}
	plan, err := DefaultRegistry().resolve(context, Selector{Only: []string{"blueclaw-payload-direct"}, Force: true})
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	joinedPlan := strings.Join(plan, ",")
	if joinedPlan != "blueclaw-payload-direct" {
		t.Fatalf("unexpected plan: %s", joinedPlan)
	}
	for _, disallowedName := range []string{"binaries", "blueclaw-runtime-base", "services", "local-llm"} {
		if strings.Contains(joinedPlan, disallowedName) {
			t.Fatalf("payload direct deploy must not include %s, got %s", disallowedName, joinedPlan)
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
	context := defaultBlueclawPlanContext("runtime-profile-missing-tools:file_deliver", "missing")

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
	context := defaultBlueclawPlanContext("runtime-profile-missing-tools:file_deliver", "ok")

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

func TestBlueclawPayloadManifestCheckReadsWorkspaceImageManifest(t *testing.T) {
	command := blueclawPayloadManifestCheckCommand(`{"runtimeName":"internkim-blueclaw-payload"}`)
	for _, expectedText := range []string{
		"payload-manifest.json",
		"/root/.blueclaw/workspace/.blueclaw/runtime/current/manifest.json",
		"debugfs -R",
		"cat /.blueclaw/runtime/current/manifest.json",
		"/var/lib/blueclaw/workspace.ext4",
	} {
		if !strings.Contains(command, expectedText) {
			t.Fatalf("expected payload manifest check command to include %q, got:\n%s", expectedText, command)
		}
	}
}

func TestOnlyBlueclawPayloadIncludesChangedBlueclawConfiguration(t *testing.T) {
	context := defaultBlueclawPlanContext("ok", "missing")
	context.SSH = blueclawPlanBoardConnection{
		runtimeContractOutput:    "ok",
		payloadManifestOutput:    "missing",
		skillsManifestOutput:     "ok",
		configurationMatchOutput: "missing",
	}

	plan, err := DefaultRegistry().resolve(context, Selector{Only: []string{"blueclaw-payload"}})
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}

	joinedPlan := strings.Join(plan, ",")
	if !strings.Contains(joinedPlan, "blueclaw-config") {
		t.Fatalf("expected changed blueclaw configuration to be planned, got %s", joinedPlan)
	}
	if strings.Index(joinedPlan, "blueclaw-config") > strings.Index(joinedPlan, "blueclaw-payload") {
		t.Fatalf("expected blueclaw configuration before payload, got %s", joinedPlan)
	}
}

func TestBlueclawPayloadRestartsServiceAfterInstall(t *testing.T) {
	connection := &recordingBoardConnection{}
	context := &Context{
		Backend: BackendSSH,
		SSH:     connection,
		Callbacks: Callbacks{
			InstallBlueclawPayloadSSH: func(context *Context) error {
				return nil
			},
		},
	}

	if errorValue := StepBlueclawPayload.Run(context); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !connection.hasCommand("systemctl restart blueclaw && systemctl is-active blueclaw") {
		t.Fatalf("expected payload install to restart blueclaw, got %+v", connection.commands)
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

func TestSkillsManifestCheckRequiresSuccessfulSyncMarker(t *testing.T) {
	command := blueclawSkillsManifestCheckCommand(`{"name":"internkim-skills"}`)
	for _, expectedText := range []string{
		skillsManifestPath,
		skillsSyncManifestPath,
	} {
		if !strings.Contains(command, expectedText) {
			t.Fatalf("expected skills manifest check command to include %q, got:\n%s", expectedText, command)
		}
	}
	if strings.Contains(command, "debugfs") {
		t.Fatalf("skills manifest check must not read a live workspace image directly:\n%s", command)
	}
}

func TestSkillsWorkspaceSyncRecordsManifestAfterSuccess(t *testing.T) {
	command := blueclawWorkspaceSkillsSyncCommand()
	syncIndex := strings.Index(command, "sync-workspace --atomic")
	markerIndex := strings.Index(command, "mv -f")
	if syncIndex < 0 || markerIndex < syncIndex || !strings.Contains(command, skillsSyncManifestPath) {
		t.Fatalf("expected successful workspace sync before atomic manifest marker update:\n%s", command)
	}
	for _, expectedText := range []string{
		"systemctl stop blueclaw",
		"systemctl is-active --quiet blueclaw",
		"systemctl kill blueclaw",
		"systemctl start blueclaw",
		"systemctl is-active blueclaw",
	} {
		if !strings.Contains(command, expectedText) {
			t.Fatalf("expected active service path to include %q:\n%s", expectedText, command)
		}
	}
}

func TestSkillsWorkspaceSyncHandlesMissingServiceAfterSync(t *testing.T) {
	command := blueclawWorkspaceSkillsSyncCommand()
	syncIndex := strings.Index(command, "sync-workspace --atomic")
	missingIndex := strings.Index(command, `if [ "$service_status" = "missing" ]; then`)
	startIndex := strings.Index(command, "systemctl start blueclaw")
	if syncIndex < 0 || missingIndex < syncIndex || startIndex < missingIndex {
		t.Fatalf("expected missing service path to sync before exiting and starting:\n%s", command)
	}
	if !strings.Contains(command[missingIndex:startIndex], "echo missing\n  exit 0") {
		t.Fatalf("expected missing service path to exit without starting:\n%s", command)
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
	runtimeContractOutput    string
	payloadManifestOutput    string
	skillsManifestOutput     string
	configurationMatchOutput string
}

type recordingBoardConnection struct {
	commands []string
}

func (connection *recordingBoardConnection) Run(command string) string {
	connection.commands = append(connection.commands, command)
	if strings.Contains(command, "systemctl restart blueclaw") {
		return "active"
	}
	return "ok"
}

func (connection *recordingBoardConnection) SCP(localPath, remotePath string) error {
	return nil
}

func (connection *recordingBoardConnection) hasCommand(fragment string) bool {
	for _, command := range connection.commands {
		if strings.Contains(command, fragment) {
			return true
		}
	}
	return false
}

func (connection blueclawPlanBoardConnection) Run(command string) string {
	switch {
	case strings.Contains(command, "cmp -s -") && strings.Contains(command, "/root/.blueclaw/config"):
		if connection.configurationMatchOutput == "" {
			return "ok"
		}
		return connection.configurationMatchOutput
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

func setupPlanIndex(plan []string, stepName string) int {
	for index, plannedStepName := range plan {
		if plannedStepName == stepName {
			return index
		}
	}
	return -1
}
