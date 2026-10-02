package blueclawworkspace

import (
	"crypto/sha256"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const maximumSkillDocumentBytes = 15000
const maximumSkillDocumentLines = 300

func requirePluginSkills(t *testing.T, repositoryRootPath string) {
	t.Helper()
	if _, errorValue := SkillRootPaths(repositoryRootPath); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func TestAgentsAssetDoesNotReferenceUnavailableSearchTool(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	document, errorValue := os.ReadFile(AgentsPath(repositoryRootPath))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if strings.Contains(string(document), "web_search") {
		t.Fatal("workspace AGENTS asset must not reference unavailable web_search tool")
	}
	if !strings.Contains(string(document), "web_fetch") {
		t.Fatal("workspace AGENTS asset must prefer available web_fetch for public page lookup")
	}
}

func TestBundledSkillDocumentsStayWithinPromptBudget(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	skillDocumentPaths := bundledSkillDocumentPaths(t, repositoryRootPath)
	if len(skillDocumentPaths) == 0 {
		t.Fatal("expected bundled skill documents")
	}

	for _, skillDocumentPath := range skillDocumentPaths {
		document, errorValue := os.ReadFile(skillDocumentPath)
		if errorValue != nil {
			t.Fatalf("expected skill document %s: %v", skillDocumentPath, errorValue)
		}
		lineCount := skillDocumentLineCount(string(document))
		if len(document) > maximumSkillDocumentBytes {
			t.Fatalf("%s is %d bytes; keep SKILL.md under %d bytes and move details into references, scripts, or assets", skillDocumentPath, len(document), maximumSkillDocumentBytes)
		}
		if lineCount > maximumSkillDocumentLines {
			t.Fatalf("%s is %d lines; keep SKILL.md under %d lines and move details into references, scripts, or assets", skillDocumentPath, lineCount, maximumSkillDocumentLines)
		}
	}
}

func bundledSkillDocumentPaths(t *testing.T, repositoryRootPath string) []string {
	t.Helper()
	patterns := []string{filepath.Join(AgentSkillsPath(repositoryRootPath), "*", "SKILL.md")}
	skillRootPaths, errorValue := SkillRootPaths(repositoryRootPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, skillRootPath := range skillRootPaths {
		patterns = append(patterns, filepath.Join(skillRootPath, "*", "SKILL.md"))
	}
	documentPaths := []string{}
	for _, pattern := range patterns {
		matches, errorValue := filepath.Glob(pattern)
		if errorValue != nil {
			t.Fatalf("expected valid glob pattern %s: %v", pattern, errorValue)
		}
		documentPaths = append(documentPaths, matches...)
	}
	return documentPaths
}

func skillDocumentLineCount(content string) int {
	if content == "" {
		return 0
	}
	lineCount := strings.Count(content, "\n")
	if !strings.HasSuffix(content, "\n") {
		lineCount++
	}
	return lineCount
}

func TestCapabilityToolAssetExistsAndIsExecutable(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	toolPath := filepath.Join(ToolsPath(repositoryRootPath), "capability")
	fileInfo, errorValue := os.Stat(toolPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if fileInfo.IsDir() {
		t.Fatalf("expected capability tool file, got directory: %s", toolPath)
	}
	if fileInfo.Mode()&0o111 == 0 {
		t.Fatalf("expected capability tool to be executable: %s", toolPath)
	}
	document, errorValue := os.ReadFile(toolPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, expectedText := range []string{"CAPABILITY_BRIDGE_URL", "/v1/tools/", "catalog", "invoke", "render"} {
		if !strings.Contains(string(document), expectedText) {
			t.Fatalf("capability tool must contain %q", expectedText)
		}
	}
}

func skillDirectoryPath(t *testing.T, repositoryRootPath string, skillName string) string {
	t.Helper()
	skillDirectories, errorValue := SkillDirectories(repositoryRootPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, skillDirectory := range skillDirectories {
		if skillDirectory.Name == skillName {
			return skillDirectory.Path
		}
	}
	t.Fatalf("no skill root provides %q", skillName)
	return ""
}

func TestArtifactSkillsDoNotUseBlueclawInternalTemporaryPath(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	requirePluginSkills(t, repositoryRootPath)
	for _, path := range officeDocumentPaths(t, repositoryRootPath) {
		document, errorValue := os.ReadFile(path)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if strings.Contains(string(document), "/workspace/.blueclaw/tmp") {
			t.Fatalf("%s must use requester temporary workspace, not /workspace/.blueclaw/tmp", path)
		}
	}
}

func TestUserFacingWorkspaceDocsDoNotExposeRuntimeInternalPaths(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	requirePluginSkills(t, repositoryRootPath)
	documentPaths := append([]string{AgentsPath(repositoryRootPath)}, officeDocumentPaths(t, repositoryRootPath)...)
	for _, documentPath := range documentPaths {
		document, errorValue := os.ReadFile(documentPath)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if strings.Contains(string(document), "/opt/blueclaw") {
			t.Fatalf("%s must not tell the model to call runtime-internal paths directly", documentPath)
		}
	}
}

func TestAgentsAssetDocumentsWorkspacePermissionBoundaries(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	document, errorValue := os.ReadFile(AgentsPath(repositoryRootPath))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	content := string(document)
	for _, expectedText := range []string{
		"Allowed workspace paths for raw terminal and file kernel tools",
		"home/<path>",
		"tmp/<artifact-slug>",
		"artifacts/<artifact-slug>",
		"/workspace/circles/<circleID>",
		"/workspace/shared/public",
		"/workspace/shared/cache/dependencies",
		"A skill's own directory is not on the workspace",
		"Linux UID, GID, supplementary groups, and file permissions",
		"Do not infer authorization from path",
		"Some parent directories allow",
	} {
		if !strings.Contains(content, expectedText) {
			t.Fatalf("workspace AGENTS asset must document %q", expectedText)
		}
	}
}

func TestCalendarAndWorkSkillsDocumentSemanticRouting(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	calendarDocument, errorValue := os.ReadFile(skillPathInTest(t, repositoryRootPath, "calendar", "SKILL.md"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	calendarContent := string(calendarDocument)
	for _, expectedText := range []string{"task_add", "task_update", "event_list", "Decide by the user's intent", "deadline-driven deliverable", "Do not mark calendar events with `[완료]`"} {
		if !strings.Contains(calendarContent, expectedText) {
			t.Fatalf("calendar skill must document mixed calendar/work routing %q", expectedText)
		}
	}

	workDocument, errorValue := os.ReadFile(skillPathInTest(t, repositoryRootPath, "internkim-task", "SKILL.md"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	workContent := string(workDocument)
	for _, expectedText := range []string{"task_list", "task_update", "event_add", "Decide by intent", "do not call the product `Flow`"} {
		if !strings.Contains(workContent, expectedText) {
			t.Fatalf("work skill must document localized semantic routing %q", expectedText)
		}
	}
}

func TestModelFacingWorkspaceDocsDoNotExposeConcretePrivatePaths(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	requirePluginSkills(t, repositoryRootPath)
	documentPaths := append(
		[]string{AgentsPath(repositoryRootPath)},
		officeDocumentPaths(t, repositoryRootPath)...,
	)
	for _, documentPath := range documentPaths {
		document, errorValue := os.ReadFile(documentPath)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if strings.Contains(string(document), "/workspace/private/people/") {
			t.Fatalf("%s must use virtual home/tmp/artifacts paths instead of concrete private paths", documentPath)
		}
	}
}

func TestBundledSkillRuntimeScriptsStayIdentical(t *testing.T) {
	skillDirectories, errorValue := SkillDirectories(filepath.Join("..", ".."))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	digestBySkill := map[string][32]byte{}
	for _, skillDirectory := range skillDirectories {
		runtimeScript, readError := os.ReadFile(filepath.Join(skillDirectory.Path, "scripts", "skill_runtime.py"))
		if readError != nil {
			continue
		}
		digestBySkill[skillDirectory.Name] = sha256.Sum256(runtimeScript)
	}
	if len(digestBySkill) < 2 {
		t.Fatalf("expected several skills to bundle skill_runtime.py, found %d", len(digestBySkill))
	}
	var referenceSkill string
	for skillName := range digestBySkill {
		if referenceSkill == "" || skillName < referenceSkill {
			referenceSkill = skillName
		}
	}
	for skillName, digest := range digestBySkill {
		if digest != digestBySkill[referenceSkill] {
			t.Fatalf("%s/scripts/skill_runtime.py drifted from %s/scripts/skill_runtime.py; bundled copies are one contract and must stay byte-identical", skillName, referenceSkill)
		}
	}
}

func TestOfficeSkillBootstrapsDependenciesFromBundledScripts(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	requirePluginSkills(t, repositoryRootPath)
	runtimePath := officePathInTest(t, repositoryRootPath, "scripts", "skill_runtime.py")
	if _, errorValue := os.Stat(runtimePath); errorValue != nil {
		t.Fatalf("office skill must bundle scripts/skill_runtime.py: %v", errorValue)
	}
	if _, errorValue := os.Stat(officePathInTest(t, repositoryRootPath, "scripts", "requirements.txt")); errorValue != nil {
		t.Fatalf("office skill must bundle scripts/requirements.txt: %v", errorValue)
	}

	for _, documentPath := range officeDocumentPaths(t, repositoryRootPath) {
		skillDocument, errorValue := os.ReadFile(documentPath)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if strings.Contains(string(skillDocument), "Do not run package installation") {
			t.Fatalf("%s must not instruct the model to handle dependency installation manually", documentPath)
		}
		if strings.Contains(string(skillDocument), "runtime is missing") {
			t.Fatalf("%s must not surface missing runtime libraries as the primary recovery path", documentPath)
		}
		if strings.Contains(string(skillDocument), `"command": "python - <<`) {
			t.Fatalf("%s must not instruct the model to bypass bundled scripts with inline Python", documentPath)
		}
	}

	runtimeScript, errorValue := os.ReadFile(runtimePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !strings.Contains(string(runtimeScript), "requirements.txt") {
		t.Fatal("office runtime script must install from requirements.txt")
	}
	if !strings.Contains(string(runtimeScript), "XDG_CACHE_HOME") {
		t.Fatal("office runtime script must keep its dependency environment under the standard cache home")
	}
	for _, hostPath := range []string{"/opt/blueclaw", "/workspace"} {
		if strings.Contains(string(runtimeScript), hostPath) {
			t.Fatalf("office runtime script must not hardcode the host path %q; a bundled skill runs wherever it is installed", hostPath)
		}
	}
	if !strings.Contains(string(runtimeScript), `"uv",`) {
		t.Fatal("office runtime script must use uv for Python dependency setup")
	}
}

func TestBundledSkillsNameNoHostEnvironmentVariable(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	requirePluginSkills(t, repositoryRootPath)
	skillRootPaths, errorValue := SkillRootPaths(repositoryRootPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, skillRootPath := range skillRootPaths {
		walkError := filepath.WalkDir(skillRootPath, func(path string, entry fs.DirEntry, walkError error) error {
			if walkError != nil || entry.IsDir() {
				return walkError
			}
			content, readError := os.ReadFile(path)
			if readError != nil {
				return readError
			}
			if strings.Contains(string(content), "BLUECLAW_") {
				t.Errorf("%s names the host that runs it; a bundled skill reads only environment variables no harness owns", path)
			}
			return nil
		})
		if walkError != nil {
			t.Fatal(walkError)
		}
	}
}

func TestArtifactSkillsDocumentGroundedQualityAndValidationWarnings(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	requirePluginSkills(t, repositoryRootPath)
	for _, skillPath := range []string{
		officePathInTest(t, repositoryRootPath, "SKILL.md"),
	} {
		document, errorValue := os.ReadFile(skillPath)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		content := string(document)
		if !strings.Contains(content, "source of truth") {
			t.Fatalf("%s must preserve supplied data as source of truth", skillPath)
		}
	}

	pdfReference, errorValue := os.ReadFile(officePathInTest(t, repositoryRootPath, "references", "pdf.md"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, expectedText := range []string{"office pdf validate", "extractable text"} {
		if !strings.Contains(string(pdfReference), expectedText) {
			t.Fatalf("pdf reference must include %q", expectedText)
		}
	}

}

func TestCalculatorSkillRunsBundledEvaluatorThroughTerminal(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	requirePluginSkills(t, repositoryRootPath)
	skillDocument, errorValue := os.ReadFile(skillPathInTest(t, repositoryRootPath, "calculator", "SKILL.md"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	skillContent := string(skillDocument)
	if !strings.Contains(skillContent, `kim.intern.tool-references: "bash"`) {
		t.Fatal("calculator skill must declare the terminal it needs in the Agent Skills metadata map")
	}
	if !strings.Contains(skillContent, "<skill>/scripts/calc.py") {
		t.Fatal("calculator skill must run the bundled evaluator script")
	}
	if strings.Contains(skillContent, "math.calculate") {
		t.Fatal("calculator skill must not reference the removed math.calculate built-in")
	}

	evaluatorScript, errorValue := os.ReadFile(skillPathInTest(t, repositoryRootPath, "calculator", "scripts", "calc.py"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, expectedText := range []string{"MAXIMUM_EXPRESSION_LENGTH = 1024", "parentheses_are_balanced", "ast.parse", "ZeroDivisionError"} {
		if !strings.Contains(string(evaluatorScript), expectedText) {
			t.Fatalf("calculator evaluator script must include %q", expectedText)
		}
	}
}

func TestPresentationBundlesPackageManifest(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	requirePluginSkills(t, repositoryRootPath)
	skillPath := officePathInTest(t, repositoryRootPath)
	packageDocument, errorValue := os.ReadFile(filepath.Join(skillPath, "assets", "package.json"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	packageContent := string(packageDocument)
	if !strings.Contains(packageContent, "playwright-core") {
		t.Fatal("presentation package manifest must include Playwright Core")
	}
	if strings.Contains(packageContent, "@marp-team/marp-cli") {
		t.Fatal("presentation package manifest must not include Marp CLI")
	}
}

func TestPresentationRevisionWorkflowEditsLatestArtifact(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	requirePluginSkills(t, repositoryRootPath)
	documentPath := officePathInTest(t, repositoryRootPath, "references", "deck.md")
	document, errorValue := os.ReadFile(documentPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	content := string(document)
	for _, expectedText := range []string{
		"latest compatible artifact in recent same-conversation posts",
		"Older PDF or Markdown files are supporting material",
		"work only in `artifacts/<deck-slug>/`",
		"`slides.html` as the canonical controller-free source",
		"`deck restore`",
		"edit it in place with targeted changes",
		"never rewrite an existing deck whole",
		"same slug",
	} {
		if !strings.Contains(content, expectedText) {
			t.Fatalf("presentation revision workflow must document %q", expectedText)
		}
	}
	if strings.Contains(content, "rewrite the file whole instead of retrying") {
		t.Fatal("presentation revision workflow must not replace an existing deck after a missed edit")
	}
	if strings.Contains(content, "verified wholesale replacement") {
		t.Fatal("presentation workflow must not leave a whole-file replacement exception for existing decks")
	}
	if strings.Contains(content, "tmp/<deck-slug>") {
		t.Fatal("presentation workflow must keep source and build output in one persistent artifact workspace")
	}
}

func TestPresentationExporterNormalizesSlideViewer(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	requirePluginSkills(t, repositoryRootPath)
	scriptPath := officePathInTest(t, repositoryRootPath, "scripts", "deck", "slide_viewer.py")
	testProgram := `
import runpy
import sys

namespace = runpy.run_path(sys.argv[1])
inject = namespace["inject_screen_slide_viewer"]
strip = namespace["strip_screen_slide_viewer"]
source = "<html><head><title>Deck</title></head><body><section>Keep me</section></body></html>"
first = inject(source)
assert first.count("<style data-internkim-slide-viewer>") == 1
assert first.count("<script data-internkim-slide-viewer>") == 1
assert inject(first) == first
partial = source.replace("</head>", "<style data-internkim-slide-viewer>stale</style></head>")
normalized = inject(partial)
assert "stale" not in normalized
assert normalized.count("<style data-internkim-slide-viewer>") == 1
assert normalized.count("<script data-internkim-slide-viewer>") == 1
duplicate = normalized.replace("</head>", "<script data-internkim-slide-viewer>duplicate</script></head>")
deduplicated = inject(duplicate)
assert "duplicate" not in deduplicated
assert deduplicated.count("<style data-internkim-slide-viewer>") == 1
assert deduplicated.count("<script data-internkim-slide-viewer>") == 1
stripped = strip(normalized)
assert "data-internkim-slide-viewer" not in stripped
assert "<section>Keep me</section>" in stripped
`
	command := exec.Command("python3", "-c", testProgram, scriptPath)
	commandOutput, errorValue := command.CombinedOutput()
	if errorValue != nil {
		t.Fatalf("presentation viewer normalization failed: %v\n%s", errorValue, commandOutput)
	}
}

func TestPresentationRestoresControllerFreeSource(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	requirePluginSkills(t, repositoryRootPath)
	scriptPath := officePathInTest(t, repositoryRootPath, "scripts", "deck", "restore_source.py")
	temporaryPath := t.TempDir()
	deliveredPath := filepath.Join(temporaryPath, "delivered.html")
	sourcePath := filepath.Join(temporaryPath, "artifacts", "deck", "slides.html")
	deliveredHTML := "<html><head><style data-internkim-slide-viewer>viewer</style></head><body><section>Editable</section><script data-internkim-slide-viewer>controller</script></body></html>"
	if errorValue := os.WriteFile(deliveredPath, []byte(deliveredHTML), 0600); errorValue != nil {
		t.Fatal(errorValue)
	}
	command := exec.Command("python3", scriptPath, deliveredPath, sourcePath)
	command.Env = append(os.Environ(), "PYTHONPATH="+officePathInTest(t, repositoryRootPath, "scripts"))
	commandOutput, errorValue := command.CombinedOutput()
	if errorValue != nil {
		t.Fatalf("presentation source restoration failed: %v\n%s", errorValue, commandOutput)
	}
	restoredDocument, errorValue := os.ReadFile(sourcePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	restoredContent := string(restoredDocument)
	if strings.Contains(restoredContent, "data-internkim-slide-viewer") || !strings.Contains(restoredContent, "<section>Editable</section>") {
		t.Fatalf("expected controller-free editable source, got %s", restoredContent)
	}
}

func TestPresentationUsesVendoredPaperlogyDesignDefaults(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	requirePluginSkills(t, repositoryRootPath)
	skillPath := officePathInTest(t, repositoryRootPath)
	fontPath := filepath.Join(skillPath, "assets", "fonts", "paperlogy")
	for _, fileName := range []string{
		"Paperlogy-4Regular.woff2",
		"Paperlogy-6SemiBold.woff2",
		"Paperlogy-7Bold.woff2",
		"Paperlogy-8ExtraBold.woff2",
		"OFL-1.1.txt",
		"README.md",
	} {
		if _, errorValue := os.Stat(filepath.Join(fontPath, fileName)); errorValue != nil {
			t.Fatalf("presentation must vendor Paperlogy asset %s: %v", fileName, errorValue)
		}
	}

	documentPaths := []string{
		filepath.Join(skillPath, "references", "deck.md"),
		filepath.Join(skillPath, "references", "deck", "webfonts.md"),
		filepath.Join(skillPath, "references", "deck", "minimal-design.md"),
	}
	for _, documentPath := range documentPaths {
		document, errorValue := os.ReadFile(documentPath)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		content := string(document)
		for _, expectedText := range []string{"Paperlogy", "WOFF2", `"Paperlogy", "Noto Sans KR", system-ui`} {
			if !strings.Contains(content, expectedText) {
				t.Fatalf("%s must document Paperlogy default %q", documentPath, expectedText)
			}
		}
		for _, forbiddenText := range []string{"Pretendard headings and body", "Freesentation body", "display text and Freesentation"} {
			if strings.Contains(content, forbiddenText) {
				t.Fatalf("%s must not keep stale default font guidance %q", documentPath, forbiddenText)
			}
		}
	}

	webfontsDocument, errorValue := os.ReadFile(filepath.Join(skillPath, "references", "deck", "webfonts.md"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, expectedText := range []string{"Web font imports are allowed", "Pretendard web font with Paperlogy fallback", "Noto Sans KR web font with Paperlogy fallback"} {
		if !strings.Contains(string(webfontsDocument), expectedText) {
			t.Fatalf("presentation webfonts reference must document webfont fallback policy %q", expectedText)
		}
	}
}

func TestPresentationVisualStylesDefineQualityGate(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	requirePluginSkills(t, repositoryRootPath)
	documentPath := officePathInTest(t, repositoryRootPath, "references", "deck", "visual-styles.md")
	document, errorValue := os.ReadFile(documentPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	content := string(document)
	for _, expectedText := range []string{
		"Visual Identity Gate",
		"Style Prompt",
		"data-visual-system",
		"data-slide-role",
		"Swiss Board",
		"Operating Memo",
		"Evidence Atlas",
		"Launch Console",
		"Risk Ledger",
		"Field Signal",
		"colored side stripes",
		"bordered cards with soft shadows",
		"repeated `.grid` plus `.card` sections",
	} {
		if !strings.Contains(content, expectedText) {
			t.Fatalf("presentation visual styles must include %q", expectedText)
		}
	}
}

func TestPresentationCompositionSeedsKeepCreativeStructure(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	requirePluginSkills(t, repositoryRootPath)
	documentPath := officePathInTest(t, repositoryRootPath, "references", "deck", "composition-seeds.md")
	document, errorValue := os.ReadFile(documentPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	content := string(document)
	for _, expectedText := range []string{
		"not templates",
		"visual system",
		"metric tile",
		"variance bar",
		"decision rail",
		"owner-date chip",
		"Board Cockpit",
		"Operating Memo",
		"Product Map",
		"Research Wall",
		"Risk Room",
		"Timeline Rail",
		"Evidence Wall",
		"Launch Control",
		"Field Report",
		"risk, evidence, response, owner",
		"not the final visual form",
	} {
		if !strings.Contains(content, expectedText) {
			t.Fatalf("presentation composition seeds must include %q", expectedText)
		}
	}
	for _, forbiddenText := range []string{"copy this template exactly", "must use this exact CSS", "always use Board Cockpit"} {
		if strings.Contains(content, forbiddenText) {
			t.Fatalf("presentation composition seeds must not overconstrain creative layout with %q", forbiddenText)
		}
	}
}

func TestPresentationRunsBuildScriptFromTaskWorkspace(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	requirePluginSkills(t, repositoryRootPath)
	skillPath := officePathInTest(t, repositoryRootPath)
	deckDocument, errorValue := os.ReadFile(filepath.Join(skillPath, "references", "deck.md"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	deckContent := string(deckDocument)
	for _, forbiddenText := range []string{"cp /workspace/skills/", " ./build.sh", "mkdir artifacts", `"arguments":`} {
		if strings.Contains(deckContent, forbiddenText) {
			t.Fatalf("deck reference must not use fragile task-local build script copying or root-relative artifact mkdir: %q", forbiddenText)
		}
	}
	for _, expectedText := range []string{"<skill>/scripts/office deck build", "FORMATS=pptx <skill>/scripts/office deck build", `"workingDirectoryPath": "artifacts/<deck-slug>"`, "then deliver", "artifacts/<deck-slug>/build/<deck-slug>.html", "artifacts/<deck-slug>/build/<deck-slug>.pptx"} {
		if !strings.Contains(deckContent, expectedText) {
			t.Fatalf("deck reference must document %q", expectedText)
		}
	}

	if _, errorValue := os.Stat(filepath.Join(skillPath, "assets", "build.sh")); !os.IsNotExist(errorValue) {
		t.Fatal("deck build script must live under scripts, not assets")
	}

	buildPath := filepath.Join(skillPath, "scripts", "deck", "build.sh")
	buildInfo, errorValue := os.Stat(buildPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if buildInfo.Mode()&0111 == 0 {
		t.Fatal("deck build script must be executable")
	}

	buildScript, errorValue := os.ReadFile(buildPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	buildContent := string(buildScript)
	if strings.Contains(buildContent, `cd "$(dirname "$0")"`) {
		t.Fatal("deck build script must run against the caller's task workspace")
	}
	for _, expectedText := range []string{"slides.html", `FORMATS="${FORMATS:-html,review}"`, "SKILL_ASSET_DIRECTORY", "${SCRIPT_DIRECTORY}/../../assets"} {
		if !strings.Contains(buildContent, expectedText) {
			t.Fatalf("deck build script must contain %q", expectedText)
		}
	}
}

func TestDeckMinimalDesignKeepsFitSafeGuidance(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	requirePluginSkills(t, repositoryRootPath)
	minimalDesign, errorValue := os.ReadFile(officePathInTest(t, repositoryRootPath, "references", "deck", "minimal-design.md"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, expectedText := range []string{"minmax(0, 1fr)", "overflow-wrap: anywhere", "line budgets", "fit-review text files"} {
		if !strings.Contains(string(minimalDesign), expectedText) {
			t.Fatalf("minimal design reference must include fit-safe guidance %q", expectedText)
		}
	}
}

func TestEverySkillComesFromAPlugin(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	requirePluginSkills(t, repositoryRootPath)
	skillDirectories, errorValue := SkillDirectories(repositoryRootPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	pluginSkillPaths, errorValue := SkillRootPaths(repositoryRootPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, skillName := range []string{"office", "calculator", "messages"} {
		if !containsSkillNamed(skillDirectories, skillName) {
			t.Fatalf("%s is in no plugin under %v", skillName, pluginSkillPaths)
		}
	}
	for _, skillDirectory := range skillDirectories {
		if !slices.Contains(pluginSkillPaths, filepath.Dir(skillDirectory.Path)) {
			t.Fatalf("%s comes from %s, which is no plugin's skills directory", skillDirectory.Name, filepath.Dir(skillDirectory.Path))
		}
	}
}

func containsSkillNamed(skillDirectories []SkillDirectory, skillName string) bool {
	for _, skillDirectory := range skillDirectories {
		if skillDirectory.Name == skillName {
			return true
		}
	}
	return false
}

func TestSkillDirectoriesRejectTheSameSkillFromTwoRoots(t *testing.T) {
	repositoryRootPath := t.TempDir()
	writeTestPlugin(t, repositoryRootPath, "a-plugin")
	writeTestPlugin(t, repositoryRootPath, "another-plugin")
	skillRootPaths, errorValue := SkillRootPaths(repositoryRootPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, rootPath := range skillRootPaths {
		if errorValue := os.MkdirAll(filepath.Join(rootPath, "twice"), 0o755); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	if _, errorValue := SkillDirectories(repositoryRootPath); errorValue == nil {
		t.Fatal("a skill provided by two roots must be reported, not silently resolved")
	}
}

func officePathInTest(t *testing.T, repositoryRootPath string, relativeParts ...string) string {
	t.Helper()
	return skillPathInTest(t, repositoryRootPath, "office", relativeParts...)
}

func officeDocumentPaths(t *testing.T, repositoryRootPath string) []string {
	t.Helper()
	referencePaths, errorValue := filepath.Glob(officePathInTest(t, repositoryRootPath, "references", "*.md"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return append([]string{officePathInTest(t, repositoryRootPath, "SKILL.md")}, referencePaths...)
}

func skillPathInTest(t *testing.T, repositoryRootPath string, skillName string, relativeParts ...string) string {
	t.Helper()
	skillDirectories, errorValue := SkillDirectories(repositoryRootPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, skillDirectory := range skillDirectories {
		if skillDirectory.Name == skillName {
			return filepath.Join(append([]string{skillDirectory.Path}, relativeParts...)...)
		}
	}
	t.Fatalf("no plugin under %s carries the %s skill", dependencyPath(repositoryRootPath), skillName)
	return ""
}
