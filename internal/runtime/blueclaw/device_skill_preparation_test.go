package blueclaw

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// No skill command installs anything and the guest sees the skills read-only,
// so a device whose refresh delivers skills without running their setup
// delivers an office skill that refuses every command.
func TestTheDeliveryRefreshPreparesTheSkillsBeforeItDeliversThem(t *testing.T) {
	refresh := BlueclawDeliveryRefreshCommand()
	workspaceSkillsPath := BlueclawWorkspacePath + "/skills"
	preparation := strings.Index(refresh, "/run/preparation/internkim-admind "+GuestSkillPreparationVerb)
	delivery := strings.Index(refresh, "rsync -a --delete "+workspaceSkillsPath+"/ "+BlueclawDeliverySkillsPath+"/")
	if preparation < 0 || delivery < 0 || preparation > delivery {
		t.Fatalf("the refresh does not run the skills' setup before it delivers them:\n%s", refresh)
	}
	for _, step := range []string{
		"mount -o loop,ro,noload " + BlueclawRootFilesystemImagePath,
		"mount --bind " + workspaceSkillsPath + ` "$preparation/root"` + BlueclawGuestDeliverySkillsPath,
		`chroot --userspec="$(stat -c %u:%g ` + workspaceSkillsPath + `)"`,
	} {
		if !strings.Contains(refresh, step) {
			t.Fatalf("the skills are not prepared in the guest's root filesystem, at the guest's path, as their owner; missing %q:\n%s", step, refresh)
		}
	}
}

// The guest's setup runs on the interpreter and finds the tools its agent's
// commands do; this holds the place to what the rootfs actually installs.
func TestTheGuestSkillsPlaceIsWhatTheRootfsInstalls(t *testing.T) {
	prepareScript, errorValue := os.ReadFile(filepath.Join("..", "..", "..", "tools", "prepare-blueclaw-runtime"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	script := string(prepareScript)
	place := GuestBundledSkillsPlace()
	searchDirectories := strings.Split(place.SearchPath, ":")
	for _, installed := range []struct{ fragment, directory string }{
		{"UV_UNMANAGED_INSTALL=/usr/local/bin", "/usr/local/bin"},
		{`"$base_directory/usr/local/bin/bun"`, "/usr/local/bin"},
		{"python3,python3-venv", filepath.Dir(place.PythonPath)},
	} {
		if !strings.Contains(script, installed.fragment) {
			t.Fatalf("the rootfs no longer installs %q, which the guest's skill setup runs", installed.fragment)
		}
		if !containsString(searchDirectories, installed.directory) {
			t.Fatalf("the guest's skill setup runs with PATH %s, which misses %s", place.SearchPath, installed.directory)
		}
	}
	if place.SkillsPath != BlueclawGuestDeliverySkillsPath {
		t.Fatalf("the guest's skills are prepared at %s and delivered at %s", place.SkillsPath, BlueclawGuestDeliverySkillsPath)
	}
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}
