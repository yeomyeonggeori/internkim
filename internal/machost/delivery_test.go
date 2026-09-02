package machost

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestTheShareCarriesExactlyWhatTheGuestOpens(t *testing.T) {
	layout, sources := deliverySourcesForTest(t)

	if errorValue := WriteDeliveryDirectory(layout, sources); errorValue != nil {
		t.Fatalf("expected the share to be written: %v", errorValue)
	}

	for _, path := range []string{
		layout.RuntimeConfigurationPath(),
		layout.PolicyPath(),
		filepath.Join(layout.DeliveryRuntimePath(), "bin", "blueclaw"),
		filepath.Join(layout.DeliveryRuntimePath(), "migrations", "001_extension.sql"),
		filepath.Join(layout.DeliveryRuntimePath(), "manifest.json"),
		filepath.Join(layout.DeliverySkillsPath(), "a-skill", "SKILL.md"),
	} {
		if _, errorValue := os.Stat(path); errorValue != nil {
			t.Fatalf("guest-init exits when it cannot open this: %s: %v", path, errorValue)
		}
	}
}

func TestTheModesAreTheOnesTheGuestNeedsToRead(t *testing.T) {
	layout, sources := deliverySourcesForTest(t)
	if errorValue := WriteDeliveryDirectory(layout, sources); errorValue != nil {
		t.Fatal(errorValue)
	}

	for path, expectedMode := range map[string]fs.FileMode{
		layout.DeliveryRuntimePath():                                   deliveryDirectoryMode,
		layout.RuntimeConfigurationPath():                              deliveryFileMode,
		filepath.Join(layout.DeliveryRuntimePath(), "bin", "blueclaw"): deliveryExecutableMode,
	} {
		fileInformation, errorValue := os.Stat(path)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if fileInformation.Mode().Perm() != expectedMode {
			t.Fatalf("virtio-fs passes uids through untranslated, so the mode is the whole access story: %s is %v, expected %v", path, fileInformation.Mode().Perm(), expectedMode)
		}
	}
}

func TestASecondWriteReplacesWhatWasThereBefore(t *testing.T) {
	layout, sources := deliverySourcesForTest(t)
	if errorValue := WriteDeliveryDirectory(layout, sources); errorValue != nil {
		t.Fatal(errorValue)
	}
	staleSkillPath := filepath.Join(layout.DeliverySkillsPath(), "a-skill", "gone-next-time.md")
	if errorValue := os.WriteFile(staleSkillPath, []byte("stale"), 0o644); errorValue != nil {
		t.Fatal(errorValue)
	}

	if errorValue := WriteDeliveryDirectory(layout, sources); errorValue != nil {
		t.Fatal(errorValue)
	}

	if _, errorValue := os.Stat(staleSkillPath); errorValue == nil {
		t.Fatal("a skill deleted upstream has to leave the share, or the guest keeps running it")
	}
}

func TestEverySkillRootReachesTheShare(t *testing.T) {
	layout, sources := deliverySourcesForTest(t)
	pluginSkillsPath := filepath.Join(t.TempDir(), "plugin", "skills")
	writeTestFile(t, filepath.Join(pluginSkillsPath, "a-plugin-skill", "SKILL.md"), "a plugin skill")
	sources.SkillPaths = append(sources.SkillPaths, pluginSkillsPath)

	if errorValue := WriteDeliveryDirectory(layout, sources); errorValue != nil {
		t.Fatal(errorValue)
	}

	for _, skillName := range []string{"a-skill", "a-plugin-skill"} {
		path := filepath.Join(layout.DeliverySkillsPath(), skillName, "SKILL.md")
		if _, errorValue := os.Stat(path); errorValue != nil {
			t.Fatalf("the roots merge into one delivered directory; a later root must not delete an earlier one: %s: %v", path, errorValue)
		}
	}
}

func deliverySourcesForTest(t *testing.T) (Layout, DeliverySources) {
	t.Helper()
	sourceRootPath := t.TempDir()
	payloadRuntimePath := filepath.Join(sourceRootPath, "runtime", "current")
	skillsPath := filepath.Join(sourceRootPath, "skills")

	writeTestFile(t, filepath.Join(payloadRuntimePath, "bin", "blueclaw"), "binary")
	writeTestFile(t, filepath.Join(payloadRuntimePath, "migrations", "001_extension.sql"), "select 1")
	writeTestFile(t, filepath.Join(payloadRuntimePath, "manifest.json"), "{}")
	writeTestFile(t, filepath.Join(skillsPath, "a-skill", "SKILL.md"), "a skill")

	return NewLayout(filepath.Join(t.TempDir(), "install"), "/tmp/bc"), DeliverySources{
		PayloadRuntimePath:       payloadRuntimePath,
		SkillPaths:               []string{skillsPath},
		RuntimeConfigurationJSON: `{"firecracker":{}}`,
		PolicyJSON:               `{"people":[]}`,
	}
}

func writeTestFile(t *testing.T, path string, contents string) {
	t.Helper()
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o755); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(path, []byte(contents), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
}
