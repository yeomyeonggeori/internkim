package blueclawworkspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
		"Allowed workspace paths for raw terminal and file tools",
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
	for _, expectedText := range []string{"React + Vite + TypeScript + Tailwind + shadcn/ui", "UI archetype", "Stitch canonical format", "browser tools", "`bun scripts/build.ts`"} {
		if !strings.Contains(content, expectedText) {
			t.Fatalf("site-prototype must document managed scaffold contract %q", expectedText)
		}
	}
	for _, forbiddenText := range []string{"tmp/<slug>", "create missing `app/package.json`", `"workingDirectoryPath": "<sourceWorkspacePath>/app"`, "dependency-free HTML + CSS + JavaScript", "Do not use React"} {
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
	for _, expectedText := range []string{`"build": "bun scripts/build.ts"`, `"react"`, `"vite"`, `"@google/design.md"`, `"@radix-ui/react-dialog"`, `"tailwindcss"`} {
		if !strings.Contains(content, expectedText) {
			t.Fatalf("vendored site scaffold package manifest must contain %q", expectedText)
		}
	}
	for _, forbiddenText := range []string{"latest", `"dependencies": {}`} {
		if strings.Contains(content, forbiddenText) {
			t.Fatalf("vendored site scaffold package manifest must not require network dependency %q", forbiddenText)
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
	if !strings.Contains(string(packageDocument), "@marp-team/marp-cli") {
		t.Fatal("simple-slides package manifest must include Marp CLI")
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
		for _, expectedText := range []string{"Paperlogy", `"Paperlogy", "Noto Sans KR", system-ui, -apple-system, BlinkMacSystemFont, sans-serif`} {
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
		"revise `presentation.md` at least once",
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
	for _, expectedText := range []string{"/workspace/skills/simple-slides/scripts/build.sh", `"workingDirectoryPath": "tmp/<deck-slug>"`, "file.promote", "tmp/<deck-slug>/build/<deck-slug>.pptx", `"destinationDirectoryPath": "artifacts/<deck-slug>"`} {
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
	for _, expectedText := range []string{"EXTRACT_NOTES_SCRIPT", "RENDER_REVIEW_SCRIPT", "SKILL_ASSET_DIRECTORY", "../assets/package.json", "BUILD_DIR", `export TMPDIR="${BUILD_PATH}/.tmp"`, `export TMP="$TMPDIR"`, `export TEMP="$TMPDIR"`, `export HOME="${TMPDIR}/home"`, `cd "$TMPDIR"`} {
		if !strings.Contains(buildContent, expectedText) {
			t.Fatalf("simple-slides build script must contain %q", expectedText)
		}
	}
}

func TestSimpleSlidesBuildAndReviewScriptsCheckFontsAndDensity(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	skillPath := filepath.Join(repositoryRootPath, "assets", "blueclaw-workspace", "skills", "simple-slides")
	buildScript, errorValue := os.ReadFile(filepath.Join(skillPath, "scripts", "build.sh"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	buildContent := string(buildScript)
	for _, expectedText := range []string{".woff2", ".woff", "font/woff2", "Embedding local fonts as base64 data URLs"} {
		if !strings.Contains(buildContent, expectedText) {
			t.Fatalf("simple-slides build script must inline local fonts with %q", expectedText)
		}
	}

	reviewScript, errorValue := os.ReadFile(filepath.Join(skillPath, "scripts", "render_review.py"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	reviewContent := string(reviewScript)
	for _, expectedText := range []string{"contentDensity", "content_density", "notTooEmpty", "notTooDense"} {
		if !strings.Contains(reviewContent, expectedText) {
			t.Fatalf("render review must include density check %q", expectedText)
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
	for _, expectedText := range []string{"simple-slides", "existing PPTX", "direct PowerPoint object editing", "editable-only", "Paperlogy"} {
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
	for _, expectedText := range []string{"Paperlogy", "DEFAULT_COLORS", "comparison", "matrix", "timeline"} {
		if !strings.Contains(createContent, expectedText) {
			t.Fatalf("pptx create script must include Paperlogy layout helper %q", expectedText)
		}
	}

	validateScript, errorValue := os.ReadFile(filepath.Join(skillPath, "scripts", "validate_pptx.py"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	validateContent := string(validateScript)
	for _, expectedText := range []string{"slide appears empty", "slide is missing a title", "Aptos", "Calibri", "excessive shape count"} {
		if !strings.Contains(validateContent, expectedText) {
			t.Fatalf("pptx validator must report design warning %q", expectedText)
		}
	}
}
