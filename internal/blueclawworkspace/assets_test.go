package blueclawworkspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const maximumSkillDocumentBytes = 15000
const maximumSkillDocumentLines = 300

func TestAgentsAssetDoesNotReferenceUnavailableSearchTool(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	document, errorValue := os.ReadFile(AgentsPath(repositoryRootPath))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if strings.Contains(string(document), "web.search") {
		t.Fatal("workspace AGENTS asset must not reference unavailable web.search tool")
	}
	if !strings.Contains(string(document), "web.fetch") {
		t.Fatal("workspace AGENTS asset must prefer available web.fetch for public page lookup")
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
	patterns := []string{
		filepath.Join(repositoryRootPath, "assets", "blueclaw-workspace", "skills", "*", "SKILL.md"),
		filepath.Join(repositoryRootPath, "assets", "blueclaw-workspace", ".agents", "skills", "*", "SKILL.md"),
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

func TestArtifactSkillsDoNotUseBlueclawInternalTemporaryPath(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	skillNames := []string{"docx", "xlsx", "pptx", "simple-slides", "pdf"}
	for _, skillName := range skillNames {
		path := filepath.Join(repositoryRootPath, "assets", "blueclaw-workspace", "skills", skillName, "SKILL.md")
		document, errorValue := os.ReadFile(path)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if strings.Contains(string(document), "/workspace/.blueclaw/tmp") {
			t.Fatalf("%s skill must use requester temporary workspace, not /workspace/.blueclaw/tmp", skillName)
		}
		if strings.Contains(string(document), "$BLUECLAW_TASK_TMP") || strings.Contains(string(document), "$BLUECLAW_REQUESTER_ARTIFACTS") {
			t.Fatalf("%s skill must use relative workspace paths in tool path fields, not shell variables", skillName)
		}
	}
}

func TestUserFacingWorkspaceDocsDoNotExposeRuntimeInternalPaths(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	documentPaths := []string{
		AgentsPath(repositoryRootPath),
		filepath.Join(repositoryRootPath, "assets", "blueclaw-workspace", "skills", "docx", "SKILL.md"),
		filepath.Join(repositoryRootPath, "assets", "blueclaw-workspace", "skills", "xlsx", "SKILL.md"),
		filepath.Join(repositoryRootPath, "assets", "blueclaw-workspace", "skills", "pptx", "SKILL.md"),
		filepath.Join(repositoryRootPath, "assets", "blueclaw-workspace", "skills", "skill-management", "SKILL.md"),
	}
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
		"/workspace/skills/<skill>/scripts/...",
		"Denied or internal paths",
		"/workspace/.blueclaw/*",
		"Concrete private POSIX paths for people",
		"/opt/*",
		"/tmp/*",
		"Some parent directories allow",
	} {
		if !strings.Contains(content, expectedText) {
			t.Fatalf("workspace AGENTS asset must document %q", expectedText)
		}
	}
}

func TestCalendarAndWorkSkillsDocumentSemanticRouting(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	calendarDocument, errorValue := os.ReadFile(filepath.Join(repositoryRootPath, "assets", "blueclaw-workspace", "skills", "calendar", "SKILL.md"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	calendarContent := string(calendarDocument)
	for _, expectedText := range []string{"task.add", "task.update", "Decide by the user's intent", "deadline-driven deliverable", "Do not mark calendar events with `[완료]`"} {
		if !strings.Contains(calendarContent, expectedText) {
			t.Fatalf("calendar skill must document mixed calendar/work routing %q", expectedText)
		}
	}

	workDocument, errorValue := os.ReadFile(filepath.Join(repositoryRootPath, "assets", "blueclaw-workspace", "skills", "internkim-flow", "SKILL.md"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	workContent := string(workDocument)
	for _, expectedText := range []string{"task.list", "task.update", "calendar.add", "Decide by intent", "do not call the product `Flow`"} {
		if !strings.Contains(workContent, expectedText) {
			t.Fatalf("work skill must document localized semantic routing %q", expectedText)
		}
	}
}

func TestModelFacingWorkspaceDocsDoNotExposeConcretePrivatePaths(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	documentPaths := []string{
		AgentsPath(repositoryRootPath),
		filepath.Join(repositoryRootPath, "assets", "blueclaw-workspace", "skills", "site-prototype", "SKILL.md"),
		filepath.Join(repositoryRootPath, "assets", "blueclaw-workspace", "skills", "docx", "SKILL.md"),
		filepath.Join(repositoryRootPath, "assets", "blueclaw-workspace", "skills", "xlsx", "SKILL.md"),
		filepath.Join(repositoryRootPath, "assets", "blueclaw-workspace", "skills", "pptx", "SKILL.md"),
		filepath.Join(repositoryRootPath, "assets", "blueclaw-workspace", "skills", "simple-slides", "SKILL.md"),
		filepath.Join(repositoryRootPath, "assets", "blueclaw-workspace", "skills", "pdf", "SKILL.md"),
	}
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

func TestSitePrototypeUsesManagedScaffoldContract(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	skillPath := filepath.Join(repositoryRootPath, "assets", "blueclaw-workspace", "skills", "site-prototype", "SKILL.md")
	document, errorValue := os.ReadFile(skillPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	content := string(document)
	for _, expectedText := range []string{"UI archetype", "Stitch canonical format", "browser capability operations", "`bun scripts/build.ts`", "artifact.review", "same URL", "app/public/site-content.json", "no build step", "build-quality.json", ".internkim/idea.md", ".internkim/artifact-brief.md", "visualReviewUnavailable", "PocketBase", "ownerIdentity", "ambiguous", "site.publish", "site.status", "the site.preview operation", "workspaceHealth", "black-on-white", "no dark navy shell", "WOFF2 assets", `format("woff2")`} {
		if !strings.Contains(content, expectedText) {
			t.Fatalf("site-prototype must document managed scaffold contract %q", expectedText)
		}
	}
	for _, forbiddenText := range []string{"tmp/<slug>", "create missing `app/package.json`", `"workingDirectoryPath": "<sourceWorkspacePath>/app"`, "warm limestone", "green secondary accents", "amber tertiary"} {
		if strings.Contains(content, forbiddenText) {
			t.Fatalf("site-prototype must not document stale site workspace pattern %q", forbiddenText)
		}
	}
}

func TestVendoredSiteScaffoldIncludesBuildManifest(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	packagePath := filepath.Join(repositoryRootPath, "assets", "blueclaw-site-scaffold", "react-vite-ts", "package.json")
	document, errorValue := os.ReadFile(packagePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	content := string(document)
	for _, expectedText := range []string{`"build": "bun scripts/build.ts"`, `"preview": "vite preview --host 0.0.0.0 --port 4173"`, `"dependencies"`, `"devDependencies"`} {
		if !strings.Contains(content, expectedText) {
			t.Fatalf("vendored site scaffold package manifest must contain %q", expectedText)
		}
	}
	for _, forbiddenText := range []string{`"latest"`, "@google/design.md", `": "^`} {
		if strings.Contains(content, forbiddenText) {
			t.Fatalf("vendored site scaffold package manifest must not use an unpinned version %q", forbiddenText)
		}
	}
}

func TestArtifactPythonSkillsBootstrapDependenciesFromBundledScripts(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	skillNames := []string{"docx", "xlsx", "pptx"}
	for _, skillName := range skillNames {
		skillPath := filepath.Join(repositoryRootPath, "assets", "blueclaw-workspace", "skills", skillName)
		runtimePath := filepath.Join(skillPath, "scripts", "skill_runtime.py")
		if _, errorValue := os.Stat(runtimePath); errorValue != nil {
			t.Fatalf("%s skill must bundle scripts/skill_runtime.py: %v", skillName, errorValue)
		}
		requirementsPath := filepath.Join(skillPath, "scripts", "requirements.txt")
		if _, errorValue := os.Stat(requirementsPath); errorValue != nil {
			t.Fatalf("%s skill must bundle scripts/requirements.txt: %v", skillName, errorValue)
		}

		skillDocument, errorValue := os.ReadFile(filepath.Join(skillPath, "SKILL.md"))
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if strings.Contains(string(skillDocument), "Do not run package installation") {
			t.Fatalf("%s skill must not instruct the model to handle dependency installation manually", skillName)
		}
		if strings.Contains(string(skillDocument), "runtime is missing") {
			t.Fatalf("%s skill must not surface missing runtime libraries as the primary recovery path", skillName)
		}
		if strings.Contains(string(skillDocument), `"command": "python - <<`) {
			t.Fatalf("%s skill must not instruct the model to bypass bundled scripts with inline Python", skillName)
		}

		createScript, errorValue := os.ReadFile(filepath.Join(skillPath, "scripts", "create_"+skillName+".py"))
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if !strings.Contains(string(createScript), "ensure_requirements(") {
			t.Fatalf("%s create script must bootstrap its own Python requirements", skillName)
		}

		runtimeScript, errorValue := os.ReadFile(runtimePath)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if !strings.Contains(string(runtimeScript), "requirements.txt") {
			t.Fatalf("%s runtime script must install from requirements.txt", skillName)
		}
		if !strings.Contains(string(runtimeScript), "/opt/blueclaw/builtin-skills-venv/bin/python") {
			t.Fatalf("%s runtime script must prefer the built-in skills Python environment", skillName)
		}
		if !strings.Contains(string(runtimeScript), `"uv",`) {
			t.Fatalf("%s runtime script must use uv for Python dependency setup", skillName)
		}
		if !strings.Contains(string(runtimeScript), "Path(sys.executable).absolute()") {
			t.Fatalf("%s runtime script must compare Python paths without resolving venv symlinks", skillName)
		}
	}
}

func TestArtifactSkillsDocumentGroundedQualityAndValidationWarnings(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	for _, skillName := range []string{"docx", "xlsx", "pdf", "site-prototype"} {
		skillPath := filepath.Join(repositoryRootPath, "assets", "blueclaw-workspace", "skills", skillName, "SKILL.md")
		document, errorValue := os.ReadFile(skillPath)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		content := string(document)
		if !strings.Contains(content, "source of truth") {
			t.Fatalf("%s skill must preserve supplied data as source of truth", skillName)
		}
	}

	docxCreateScript, errorValue := os.ReadFile(filepath.Join(repositoryRootPath, "assets", "blueclaw-workspace", "skills", "docx", "scripts", "create_docx.py"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, expectedText := range []string{"eastAsia", "set_table_borders", "columnWidthsInches"} {
		if !strings.Contains(string(docxCreateScript), expectedText) {
			t.Fatalf("docx create script must include %q", expectedText)
		}
	}

	xlsxCreateScript, errorValue := os.ReadFile(filepath.Join(repositoryRootPath, "assets", "blueclaw-workspace", "skills", "xlsx", "scripts", "create_xlsx.py"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, expectedText := range []string{"default_freeze_panes", "auto_filter_reference", "create_thin_border", "heading"} {
		if !strings.Contains(string(xlsxCreateScript), expectedText) {
			t.Fatalf("xlsx create script must include %q", expectedText)
		}
	}

	for _, validationScriptPath := range []string{
		filepath.Join(repositoryRootPath, "assets", "blueclaw-workspace", "skills", "docx", "scripts", "validate_docx.py"),
		filepath.Join(repositoryRootPath, "assets", "blueclaw-workspace", "skills", "xlsx", "scripts", "validate_xlsx.py"),
		filepath.Join(repositoryRootPath, "assets", "blueclaw-workspace", "skills", "pdf", "scripts", "validate_pdf.py"),
	} {
		document, errorValue := os.ReadFile(validationScriptPath)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if !strings.Contains(string(document), "warningCount") {
			t.Fatalf("%s must report warningCount", validationScriptPath)
		}
	}

	xlsxValidationScript, errorValue := os.ReadFile(filepath.Join(repositoryRootPath, "assets", "blueclaw-workspace", "skills", "xlsx", "scripts", "validate_xlsx.py"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, expectedText := range []string{"offRowFormulaCount", "headerRow", "titleRowDetected"} {
		if !strings.Contains(string(xlsxValidationScript), expectedText) {
			t.Fatalf("xlsx validation script must include %q", expectedText)
		}
	}

	pdfSkillDocument, errorValue := os.ReadFile(filepath.Join(repositoryRootPath, "assets", "blueclaw-workspace", "skills", "pdf", "SKILL.md"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, expectedText := range []string{"validate_pdf.py", "extractable PDF text", "computed total equals"} {
		if !strings.Contains(string(pdfSkillDocument), expectedText) {
			t.Fatalf("pdf skill must include %q", expectedText)
		}
	}
	for _, fileName := range []string{"skill_runtime.py", "requirements.txt", "validate_pdf.py"} {
		if _, errorValue := os.Stat(filepath.Join(repositoryRootPath, "assets", "blueclaw-workspace", "skills", "pdf", "scripts", fileName)); errorValue != nil {
			t.Fatalf("pdf skill must bundle scripts/%s: %v", fileName, errorValue)
		}
	}

	siteSkillDocument, errorValue := os.ReadFile(filepath.Join(repositoryRootPath, "assets", "blueclaw-workspace", "skills", "site-prototype", "SKILL.md"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, expectedText := range []string{"must-show source content", "rendered text", "source checklist"} {
		if !strings.Contains(string(siteSkillDocument), expectedText) {
			t.Fatalf("site-prototype skill must include %q", expectedText)
		}
	}
}

func TestBuiltinSkillDependenciesArePreinstalledInRuntimeBase(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	requirementsDocument, errorValue := os.ReadFile(filepath.Join(repositoryRootPath, "assets", "blueclaw-runtime", "builtin-skills-requirements.txt"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	requirements := string(requirementsDocument)
	for _, packageName := range []string{"fpdf2", "openpyxl", "pypdf", "python-docx", "python-pptx"} {
		if !strings.Contains(requirements, packageName) {
			t.Fatalf("builtin skill requirements must include %s", packageName)
		}
	}

	prepareScript, errorValue := os.ReadFile(filepath.Join(repositoryRootPath, "tools", "prepare-blueclaw-runtime"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	script := string(prepareScript)
	for _, fragment := range []string{"UV_UNMANAGED_INSTALL=/usr/local/bin", "/opt/blueclaw/builtin-skills-venv", "builtin-skills-requirements.txt"} {
		if !strings.Contains(script, fragment) {
			t.Fatalf("prepare script must contain %q", fragment)
		}
	}
}

func TestSimpleSlidesBundlesPackageManifest(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	skillPath := filepath.Join(repositoryRootPath, "assets", "blueclaw-workspace", "skills", "simple-slides")
	packageDocument, errorValue := os.ReadFile(filepath.Join(skillPath, "assets", "package.json"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	packageContent := string(packageDocument)
	if !strings.Contains(packageContent, "playwright-core") {
		t.Fatal("simple-slides package manifest must include Playwright Core")
	}
	if strings.Contains(packageContent, "@marp-team/marp-cli") {
		t.Fatal("simple-slides package manifest must not include Marp CLI")
	}
}

func TestSimpleSlidesUsesVendoredPaperlogyDesignDefaults(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	skillPath := filepath.Join(repositoryRootPath, "assets", "blueclaw-workspace", "skills", "simple-slides")
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
			t.Fatalf("simple-slides must vendor Paperlogy asset %s: %v", fileName, errorValue)
		}
	}

	documentPaths := []string{
		filepath.Join(skillPath, "SKILL.md"),
		filepath.Join(skillPath, "assets", "webfonts.md"),
		filepath.Join(skillPath, "assets", "minimal-design.md"),
	}
	for _, documentPath := range documentPaths {
		document, errorValue := os.ReadFile(documentPath)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		content := string(document)
		for _, expectedText := range []string{"Paperlogy", "WOFF2", `"Paperlogy", "Noto Sans KR", system-ui, -apple-system, BlinkMacSystemFont`, `"Noto Color Emoji"`} {
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
}

func TestSimpleSlidesDocumentsBeautifulDeckContract(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	skillPath := filepath.Join(repositoryRootPath, "assets", "blueclaw-workspace", "skills", "simple-slides")
	document, errorValue := os.ReadFile(filepath.Join(skillPath, "SKILL.md"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	content := string(document)
	for _, expectedText := range []string{
		"deck archetype",
		"pitch, research report, executive briefing, education, portfolio, product proposal, or status report",
		"title thesis, section divider, comparison, matrix, timeline, evidence card, recommendation, and closing ask",
		"slide-review.json",
		"contact sheets",
		"fit-review-XX.md",
		"expected visible text",
		"deck-brief.md",
		"story spine",
		"Reject shallow content",
		"worked example",
		"rendered image evidence",
		"revise `slides.html`",
		"artifact.review",
		"`files` array",
		"Do not spend delivery budget creating or attaching internal review-decision files",
		"not a delivery blocker",
		"Do not use emoji as functional icons or bullets",
		"HTML-first",
	} {
		if !strings.Contains(content, expectedText) {
			t.Fatalf("simple-slides must document beautiful deck contract %q", expectedText)
		}
	}
}

func TestSimpleSlidesRunsBuildScriptFromTaskWorkspace(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	skillPath := filepath.Join(repositoryRootPath, "assets", "blueclaw-workspace", "skills", "simple-slides")
	skillDocument, errorValue := os.ReadFile(filepath.Join(skillPath, "SKILL.md"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	skillContent := string(skillDocument)
	for _, forbiddenText := range []string{"cp /workspace/skills/simple-slides", "/workspace/skills/simple-slides/assets/build.sh", " ./build.sh", "mkdir artifacts"} {
		if strings.Contains(skillContent, forbiddenText) {
			t.Fatalf("simple-slides must not use fragile task-local build script copying or root-relative artifact mkdir: %q", forbiddenText)
		}
	}
	for _, expectedText := range []string{"/workspace/skills/simple-slides/scripts/build.sh", `"workingDirectoryPath": "tmp/<deck-slug>"`, "file.deliver", "tmp/<deck-slug>/build/<deck-slug>.pptx"} {
		if !strings.Contains(skillContent, expectedText) {
			t.Fatalf("simple-slides must document %q", expectedText)
		}
	}

	if _, errorValue := os.Stat(filepath.Join(skillPath, "assets", "build.sh")); !os.IsNotExist(errorValue) {
		t.Fatal("simple-slides build script must live under scripts, not assets")
	}

	buildPath := filepath.Join(skillPath, "scripts", "build.sh")
	buildInfo, errorValue := os.Stat(buildPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if buildInfo.Mode()&0111 == 0 {
		t.Fatal("simple-slides build script must be executable")
	}

	buildScript, errorValue := os.ReadFile(buildPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	buildContent := string(buildScript)
	if strings.Contains(buildContent, `cd "$(dirname "$0")"`) {
		t.Fatal("simple-slides build script must run against the caller's task workspace")
	}
	if strings.Contains(buildContent, "command -v marp") {
		t.Fatal("simple-slides build script must not select ambiguous global Marp")
	}
	for _, expectedText := range []string{"slides.html", "HTML_EXPORT_SCRIPT", "HTML_RENDER_SCRIPT", "RENDER_REVIEW_SCRIPT", "SKILL_ASSET_DIRECTORY", "../assets/package.json", "BUILD_DIR", `export TMPDIR="${BUILD_PATH}/.tmp"`, `export TMP="$TMPDIR"`, `export TEMP="$TMPDIR"`, `export HOME="${TMPDIR}/home"`, `${WORK_DIR}/.skill-env/simple-slides`, "NODE_RUNTIME_BUN_INSTALL", "NODE_RUNTIME_BUN_CACHE", `export BUN_INSTALL="$NODE_RUNTIME_BUN_INSTALL"`, `export BUN_INSTALL_CACHE_DIR="$NODE_RUNTIME_BUN_CACHE"`, "playwright-core"} {
		if !strings.Contains(buildContent, expectedText) {
			t.Fatalf("simple-slides build script must contain %q", expectedText)
		}
	}
	for _, forbiddenText := range []string{"presentation.md", "EXTRACT_NOTES_SCRIPT", "REVIEW_STRICT", `cd "$TMPDIR"`, "/workspace/shared/cache/dependencies/bun", "BLUECLAW_REQUESTER_TMP", "is older than DESIGN.md"} {
		if strings.Contains(buildContent, forbiddenText) {
			t.Fatalf("simple-slides build script must not contain old workflow fragment %q", forbiddenText)
		}
	}
}

func TestSimpleSlidesBuildAndReviewScriptsCheckFontsAndDensity(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	skillPath := filepath.Join(repositoryRootPath, "assets", "blueclaw-workspace", "skills", "simple-slides")
	htmlExportScript, errorValue := os.ReadFile(filepath.Join(skillPath, "scripts", "html_export.py"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	htmlExportContent := string(htmlExportScript)
	for _, expectedText := range []string{".woff2", ".woff", "font/woff2", "base64_data_url", "inline_local_fonts", "inject_vendored_paperlogy_fallback", "PaperlogyLocal", "add_paperlogy_local_to_font_family_lists", "write_image_backed_pptx", "data-internkim-slide-viewer", "bespoke-marp-parent", "bespoke-marp-osc", "internkim-deck-scale", "data-lucide", "lucideIcon", "bespoke-marp-tooltip", "Next slide (→ / Space / PageDown)", "Overview (O)", "Presenter view (P)", "Exit fullscreen (F)", "minimize", "window.opener.postMessage", "body[data-bespoke-view=\"presenter\"] .bespoke-marp-osc button:not", "width: 1600px", "height: 900px", "resolve_paperlogy_alias", "slideMasters/slideMaster1.xml", "slideLayouts/slideLayout1.xml", "theme/theme1.xml"} {
		if !strings.Contains(htmlExportContent, expectedText) {
			t.Fatalf("simple-slides HTML export script must inline fonts, present HTML as slides, and export valid PPTX with %q", expectedText)
		}
	}
	if strings.Contains(htmlExportContent, "@layer internkim-fonts") {
		t.Fatal("simple-slides font fallback must use plain @font-face rules for browser compatibility")
	}
	for _, forbiddenText := range []string{"is older than DESIGN.md"} {
		if strings.Contains(htmlExportContent, forbiddenText) {
			t.Fatalf("simple-slides HTML export script must not contain old workflow fragment %q", forbiddenText)
		}
	}

	htmlRenderScript, errorValue := os.ReadFile(filepath.Join(skillPath, "scripts", "html_render.mjs"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	htmlRenderContent := string(htmlRenderScript)
	for _, expectedText := range []string{"page.pdf", "preferCSSPageSize", "printBackground", "document.fonts.ready", "locator(\"section\")"} {
		if !strings.Contains(htmlRenderContent, expectedText) {
			t.Fatalf("simple-slides HTML render script must render text-preserving PDF and slide PNGs with %q", expectedText)
		}
	}

	reviewScript, errorValue := os.ReadFile(filepath.Join(skillPath, "scripts", "render_review.py"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	reviewContent := string(reviewScript)
	for _, expectedText := range []string{"contentDensity", "content_density", "notTooEmpty", "notTooDense", "fit-review.json", "fit-review-XX.md", "textOverflowRisk", "frameFitRisk", "fitReviewFilename"} {
		if !strings.Contains(reviewContent, expectedText) {
			t.Fatalf("render review must include density check %q", expectedText)
		}
	}
	if strings.Contains(reviewContent, `return 0 if report["passed"] else 1`) {
		t.Fatalf("render review warnings must remain LLM review input, not fail the build")
	}
	for _, forbiddenText := range []string{"presentation.md", "remove_front_matter", "split(r\"(?m)^\\s*---\\s*$\""} {
		if strings.Contains(reviewContent, forbiddenText) {
			t.Fatalf("render review must stay on HTML section source and not contain %q", forbiddenText)
		}
	}

	acceptScript, errorValue := os.ReadFile(filepath.Join(skillPath, "scripts", "accept_review.py"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	acceptContent := string(acceptScript)
	for _, expectedText := range []string{"review-decision.json", "inspectedEvidence", "remainingNotes", "acceptedWarnings", "string_set_field", "should be a list"} {
		if !strings.Contains(acceptContent, expectedText) {
			t.Fatalf("review acceptance script must include %q", expectedText)
		}
	}
	for _, forbiddenText := range []string{"blocking issues remain", "raise ValueError(\"contact sheets were not inspected"} {
		if strings.Contains(acceptContent, forbiddenText) {
			t.Fatalf("review acceptance must report review warnings without hard-blocking delivery on %q", forbiddenText)
		}
	}

	minimalDesign, errorValue := os.ReadFile(filepath.Join(skillPath, "assets", "minimal-design.md"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	minimalDesignContent := string(minimalDesign)
	for _, expectedText := range []string{"minmax(0, 1fr)", "overflow-wrap: anywhere", "line budgets", "fit-review text files"} {
		if !strings.Contains(minimalDesignContent, expectedText) {
			t.Fatalf("minimal design reference must include fit-safe guidance %q", expectedText)
		}
	}
}

func TestPPTXSkillRoutesBeautifulNewDecksToSimpleSlides(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	skillPath := filepath.Join(repositoryRootPath, "assets", "blueclaw-workspace", "skills", "pptx")
	document, errorValue := os.ReadFile(filepath.Join(skillPath, "SKILL.md"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	content := string(document)
	for _, expectedText := range []string{"simple-slides", "existing PPTX", "direct PowerPoint object editing", "editable-only", "Paperlogy", "hybrid deck", "backgroundImage", "editableTexts", "artifact.review"} {
		if !strings.Contains(content, expectedText) {
			t.Fatalf("pptx skill must document routing and editability policy %q", expectedText)
		}
	}
}

func TestPPTXScriptsUsePaperlogyAndDesignWarnings(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	skillPath := filepath.Join(repositoryRootPath, "assets", "blueclaw-workspace", "skills", "pptx")
	createScript, errorValue := os.ReadFile(filepath.Join(skillPath, "scripts", "create_pptx.py"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	createContent := string(createScript)
	for _, expectedText := range []string{"Paperlogy", "DEFAULT_COLORS", "comparison", "matrix", "timeline", "backgroundImage", "editableTexts"} {
		if !strings.Contains(createContent, expectedText) {
			t.Fatalf("pptx create script must include Paperlogy layout helper %q", expectedText)
		}
	}

	validateScript, errorValue := os.ReadFile(filepath.Join(skillPath, "scripts", "validate_pptx.py"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	validateContent := string(validateScript)
	for _, expectedText := range []string{"slide appears empty", "slide is missing a title", "Aptos", "Calibri", "excessive shape count", "hybrid background", "editable overlay out of bounds"} {
		if !strings.Contains(validateContent, expectedText) {
			t.Fatalf("pptx validator must report design warning %q", expectedText)
		}
	}
}
