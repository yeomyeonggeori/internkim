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
		"/workspace/private/people/<yourPersonID>",
		"tmp/<artifact-slug>",
		"artifacts/<artifact-slug>",
		"/workspace/circles/<circleID>",
		"/workspace/shared/public",
		"/workspace/shared/cache/dependencies",
		"/workspace/skills/<skill>/scripts/...",
		"Denied or internal paths",
		"/workspace/.blueclaw/*",
		"/opt/*",
		"/tmp/*",
		"may allow path traversal without directory listing",
	} {
		if !strings.Contains(content, expectedText) {
			t.Fatalf("workspace AGENTS asset must document %q", expectedText)
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
	for _, expectedText := range []string{"EXTRACT_NOTES_SCRIPT", "RENDER_REVIEW_SCRIPT", "SKILL_ASSET_DIRECTORY", "../assets/package.json", "BUILD_DIR", `export TMPDIR="${BUILD_PATH}/.tmp"`, `export TMP="$TMPDIR"`, `export TEMP="$TMPDIR"`, `cd "$TMPDIR"`} {
		if !strings.Contains(buildContent, expectedText) {
			t.Fatalf("simple-slides build script must contain %q", expectedText)
		}
	}
}
