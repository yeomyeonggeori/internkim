package blueclawworkspace

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

const hostSkillsImagePath = "/opt/internkim/skills"

var systemFontPathPattern = regexp.MustCompile(`/usr/share/fonts/[A-Za-z0-9._/-]+`)

func skillScriptSources(t *testing.T, skillDirectory SkillDirectory) string {
	t.Helper()
	scriptPaths, errorValue := filepath.Glob(filepath.Join(skillDirectory.Path, "scripts", "*.py"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	sources := strings.Builder{}
	for _, scriptPath := range scriptPaths {
		script, readError := os.ReadFile(scriptPath)
		if readError != nil {
			t.Fatal(readError)
		}
		sources.Write(script)
	}
	return sources.String()
}

func hostDockerfile(t *testing.T, repositoryRootPath string) string {
	t.Helper()
	document, errorValue := os.ReadFile(filepath.Join(repositoryRootPath, "host", "Dockerfile"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return string(document)
}

func hostEntrypoint(t *testing.T, repositoryRootPath string) string {
	t.Helper()
	document, errorValue := os.ReadFile(filepath.Join(repositoryRootPath, "host", "entrypoint.sh"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return string(document)
}

func skillsDeclaringRequirements(t *testing.T, repositoryRootPath string) []SkillDirectory {
	t.Helper()
	requirePluginSkills(t, repositoryRootPath)
	skillDirectories, errorValue := SkillDirectories(repositoryRootPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	declaring := []SkillDirectory{}
	for _, skillDirectory := range skillDirectories {
		if _, statError := os.Stat(filepath.Join(skillDirectory.Path, "scripts", "requirements.txt")); statError == nil {
			declaring = append(declaring, skillDirectory)
		}
	}
	return declaring
}

func TestHostImageResolvesEveryBundledSkillRequirement(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	declaring := skillsDeclaringRequirements(t, repositoryRootPath)
	if len(declaring) == 0 {
		t.Fatal("expected bundled skills to declare scripts/requirements.txt")
	}
	dockerfile := hostDockerfile(t, repositoryRootPath)

	if !strings.Contains(dockerfile, "COPY .dependency/internkim-plugin/skills "+hostSkillsImagePath) {
		t.Fatalf("host Dockerfile must copy the bundled skills to %s", hostSkillsImagePath)
	}
	requirementsGlob := hostSkillsImagePath + "/*/scripts/requirements.txt"
	if !strings.Contains(dockerfile, requirementsGlob) {
		t.Fatalf("host Dockerfile must resolve %s; listing the packages by hand is a second place to forget one", requirementsGlob)
	}
	if !strings.Contains(dockerfile, "uv pip install --python /opt/internkim/document-venv/bin/python") {
		t.Fatal("host Dockerfile must install the resolved requirements into the interpreter capabilityd reads")
	}
	if !strings.Contains(dockerfile, `printf '#!/bin/sh\nexec /opt/internkim/document-venv/bin/python3 "$@"\n' > /usr/local/bin/python3`) {
		t.Fatal(`host Dockerfile must put that interpreter on the requester's PATH as /usr/local/bin/python3`)
	}
}

func TestHostImageCarriesTheFontEveryPDFSkillLooksFor(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	sharedFontPaths := map[string]bool{}
	embeddingSkillCount := 0
	for _, skillDirectory := range skillsDeclaringRequirements(t, repositoryRootPath) {
		scripts := skillScriptSources(t, skillDirectory)
		if !strings.Contains(scripts, "add_font(") {
			continue
		}
		declared := map[string]bool{}
		for _, fontPath := range systemFontPathPattern.FindAllString(scripts, -1) {
			declared[fontPath] = true
		}
		if len(declared) == 0 {
			t.Fatalf("%s embeds a font into a PDF but names no system font path", skillDirectory.Name)
		}
		embeddingSkillCount++
		if embeddingSkillCount == 1 {
			sharedFontPaths = declared
			continue
		}
		for fontPath := range sharedFontPaths {
			if !declared[fontPath] {
				delete(sharedFontPaths, fontPath)
			}
		}
	}
	if embeddingSkillCount < 2 {
		t.Fatalf("expected several skills to embed a system font into a PDF, found %d", embeddingSkillCount)
	}
	if len(sharedFontPaths) == 0 {
		t.Fatal("the skills that embed a font no longer share a system path; the host image cannot satisfy them with one package")
	}

	entrypoint := hostEntrypoint(t, repositoryRootPath)
	checkedFontPath := shellAssignment(entrypoint, "koreanCapableFontPath")
	if checkedFontPath == "" {
		t.Fatal("host entrypoint must name the Korean-capable font path it checks for")
	}
	if !sharedFontPaths[checkedFontPath] {
		t.Fatalf("host entrypoint checks %s, which is not among the paths every font-embedding skill looks for (%s)", checkedFontPath, strings.Join(sortedKeys(sharedFontPaths), ", "))
	}
	if !strings.Contains(hostDockerfile(t, repositoryRootPath), "fonts-nanum") {
		t.Fatalf("host Dockerfile must install the package that provides %s", checkedFontPath)
	}
}

func TestHostEntrypointRefusesToStartWithoutWhatTheSkillsRun(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	entrypoint := hostEntrypoint(t, repositoryRootPath)
	declaredPrograms := strings.Fields(shellAssignment(entrypoint, "programsTheBundledSkillsRun"))
	declared := map[string]bool{}
	for _, programName := range declaredPrograms {
		declared[programName] = true
	}

	skillPath := skillDirectoryPath(t, repositoryRootPath, "presentation")
	renderScript, errorValue := os.ReadFile(filepath.Join(skillPath, "scripts", "html_render.mjs"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !strings.Contains(string(renderScript), `"/usr/bin/chromium"`) {
		t.Fatal("the presentation renderer no longer looks for /usr/bin/chromium; the host check needs the new name")
	}
	runtimeScript, errorValue := os.ReadFile(filepath.Join(skillPath, "scripts", "skill_runtime.py"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !strings.Contains(string(runtimeScript), `"uv",`) {
		t.Fatal("the skill runtime no longer bootstraps with uv; the host check needs the new name")
	}

	for _, programName := range []string{"python3", "bun", "uv", "chromium"} {
		if !declared[programName] {
			t.Fatalf("host entrypoint must refuse to start without %s; a skill that cannot reach it delivers a document with its Hangul or its rendered evidence missing", programName)
		}
	}
	if !strings.Contains(entrypoint, "this image carries no ${programThisImageCarries}") {
		t.Fatal("host entrypoint must name the missing program rather than failing anonymously")
	}
}

func TestBundledSkillRequirementsCarryNoVersionSpecifier(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	runtimeScript, errorValue := os.ReadFile(filepath.Join(skillDirectoryPath(t, repositoryRootPath, "pdf"), "scripts", "skill_runtime.py"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !strings.Contains(string(runtimeScript), "importlib.metadata.distribution(package_name)") {
		t.Fatal("skill_runtime.py no longer decides by distribution name alone; this guard can go once it compares versions")
	}

	specifierPattern := regexp.MustCompile(`[=<>!~]`)
	for _, skillDirectory := range skillsDeclaringRequirements(t, repositoryRootPath) {
		requirementsPath := filepath.Join(skillDirectory.Path, "scripts", "requirements.txt")
		document, readError := os.ReadFile(requirementsPath)
		if readError != nil {
			t.Fatal(readError)
		}
		for _, line := range strings.Split(string(document), "\n") {
			requirement := strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
			if requirement == "" {
				continue
			}
			if specifierPattern.MatchString(requirement) {
				t.Fatalf("%s pins %q, but skill_runtime.py only checks that a distribution of that name is importable; a host that preinstalled another version would run the skill against it without saying so", requirementsPath, requirement)
			}
		}
	}
}

func TestHostImagePinsTheUVReleaseTheDeviceRootfsInstalls(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	hostVersion := dockerfileArgument(hostDockerfile(t, repositoryRootPath), "UV_VERSION")
	if hostVersion == "" {
		t.Fatal("host Dockerfile declares no ARG UV_VERSION")
	}
	prepareScript, errorValue := os.ReadFile(filepath.Join(repositoryRootPath, "tools", "prepare-blueclaw-runtime"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	devicePin := regexp.MustCompile(`blueclaw_uv_version="\$\{BLUECLAW_UV_VERSION:-([0-9.]+)\}"`).FindStringSubmatch(string(prepareScript))
	if devicePin == nil {
		t.Fatal("prepare-blueclaw-runtime no longer pins a uv version the host can match")
	}
	if hostVersion != devicePin[1] {
		t.Fatalf("host image installs uv %s while the device rootfs installs %s; one skill environment, one release", hostVersion, devicePin[1])
	}
}

func shellAssignment(script string, name string) string {
	for _, line := range strings.Split(script, "\n") {
		value, isAssignment := strings.CutPrefix(strings.TrimSpace(line), name+"=")
		if !isAssignment {
			continue
		}
		return strings.Trim(strings.TrimSpace(value), `"`)
	}
	return ""
}

func dockerfileArgument(dockerfile string, name string) string {
	for _, line := range strings.Split(dockerfile, "\n") {
		value, isDeclared := strings.CutPrefix(strings.TrimSpace(line), "ARG "+name+"=")
		if isDeclared {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
