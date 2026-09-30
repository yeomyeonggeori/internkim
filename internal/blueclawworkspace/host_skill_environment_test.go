package blueclawworkspace

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
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

func TestHostImageInstallsOnlyTheDocumentConversionRequirementsIntoItsVenv(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	dockerfile := hostDockerfile(t, repositoryRootPath)

	if !strings.Contains(dockerfile, "COPY .dependency/internkim-plugin/skills "+hostSkillsImagePath) {
		t.Fatalf("host Dockerfile must copy the bundled skills to %s", hostSkillsImagePath)
	}
	if !strings.Contains(dockerfile, "--requirement /opt/internkim/document-conversion/requirements.txt") {
		t.Fatal("host Dockerfile must install assets/document-conversion/requirements.txt into the interpreter capabilityd reads")
	}
	if strings.Contains(dockerfile, hostSkillsImagePath+"/*/scripts/requirements.txt") {
		t.Fatal("host Dockerfile must leave the skills' requirements to the skills, which resolve them on first use")
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
			if _, carriesNoHangul := fontPathsThatCarryNoHangul[fontPath]; carriesNoHangul {
				continue
			}
			declared[fontPath] = true
		}
		if len(declared) == 0 {
			t.Fatalf("%s embeds a font into a PDF but names no system font path that carries Hangul", skillDirectory.Name)
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

	if !sharedFontPaths[blueclaw.CompanyPackageDocumentFontPath] {
		t.Fatalf("the .deb carries its Hangul font at %s and not every font-embedding skill looks there (they share %s)",
			blueclaw.CompanyPackageDocumentFontPath, strings.Join(sortedKeys(sharedFontPaths), ", "))
	}

	entrypoint := hostEntrypoint(t, repositoryRootPath)
	checkedFontPath := shellAssignment(entrypoint, "koreanCapableFontPath")
	if checkedFontPath == "" {
		t.Fatal("host entrypoint must name the Korean-capable font path it checks for")
	}
	if !sharedFontPaths[checkedFontPath] {
		t.Fatalf("host entrypoint checks %s, which is not among the Hangul-carrying paths every font-embedding skill looks for (%s)", checkedFontPath, strings.Join(sortedKeys(sharedFontPaths), ", "))
	}
	if !strings.Contains(hostDockerfile(t, repositoryRootPath), "fonts-nanum") {
		t.Fatalf("host Dockerfile must install the package that provides %s", checkedFontPath)
	}
}

func TestHostImageBuildRefusesAnIncompleteSkillEnvironment(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	entrypointPath := filepath.Join(repositoryRootPath, "host", "entrypoint.sh")
	entrypoint := hostEntrypoint(t, repositoryRootPath)
	stubDirectory := t.TempDir()
	programNames := append(strings.Fields(shellAssignment(entrypoint, "programsThisScriptRuns")), "python3", "bun", "uv", "cat")
	for _, programName := range programNames {
		stubPath := filepath.Join(stubDirectory, programName)
		if errorValue := os.WriteFile(stubPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); errorValue != nil {
			t.Fatal(errorValue)
		}
	}

	command := exec.Command("sh", entrypointPath, "--check-programs")
	command.Env = []string{"PATH=" + stubDirectory}
	output, errorValue := command.CombinedOutput()
	if errorValue == nil {
		t.Fatalf("--check-programs accepted an image with no Korean font:\n%s", output)
	}
	for _, expectedText := range []string{"carries no Korean-capable font", "install fonts-nanum"} {
		if !strings.Contains(string(output), expectedText) {
			t.Fatalf("--check-programs must say %q so the build failure names what to install; it said:\n%s", expectedText, output)
		}
	}
}

func TestARunningHostReportsAnIncompleteSkillEnvironmentInsteadOfTakingTheMessengerDown(t *testing.T) {
	repositoryRootPath := filepath.Join("..", "..")
	entrypoint := hostEntrypoint(t, repositoryRootPath)

	collector := shellFunctionBody(t, entrypoint, "whatTheBundledSkillsAreMissing")
	if strings.Contains(collector, "exit ") {
		t.Fatal("whatTheBundledSkillsAreMissing must collect what is missing, not exit; the image build decides what to do with it")
	}
	for _, expectedName := range []string{"python3", "bun", "uv"} {
		if !strings.Contains(entrypoint, expectedName) {
			t.Fatalf("host entrypoint must look for %s, which the bundled skills run", expectedName)
		}
	}

	relayIndex := strings.Index(entrypoint, "keepRelayRunning &")
	if relayIndex < 0 {
		t.Fatal("host entrypoint no longer starts the relay in the background; this guard needs the new shape")
	}
	reportIndex := strings.Index(entrypoint, `echo "[host] this box `)
	if reportIndex < 0 {
		t.Fatal("a running host must say what the bundled skills are missing")
	}
	if reportIndex < relayIndex {
		t.Fatal("the skill environment report must come after the relay is started; the relay is what answers when the agent cannot, and a company that cannot be talked to cannot be told what is wrong with it")
	}
	if !strings.Contains(entrypoint, `echo "[host] up, incomplete`) {
		t.Fatal("the last line a person reads must say the box came up incomplete, or the report scrolls away")
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

func shellFunctionBody(t *testing.T, script string, name string) string {
	t.Helper()
	start := strings.Index(script, name+"() {")
	if start < 0 {
		t.Fatalf("host entrypoint defines no %s()", name)
	}
	end := strings.Index(script[start:], "\n}\n")
	if end < 0 {
		t.Fatalf("%s() has no closing brace on its own line", name)
	}
	return script[start : start+end]
}
