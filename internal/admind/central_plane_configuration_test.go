package admind

import (
	"os"
	"path/filepath"
	"testing"

	"gitlab.com/eastriver/internkim/internal/centralplane"
)

func writeSettingForTest(t *testing.T, directory string, name string, value string) string {
	t.Helper()
	path := filepath.Join(directory, name)
	if errorValue := os.WriteFile(path, []byte(value+"\n"), 0o644); errorValue != nil {
		t.Fatal(errorValue)
	}
	return path
}

func TestACompanyNamesItsOwnRecordInFiles(t *testing.T) {
	directory := t.TempDir()
	configuration := Configuration{
		CentralPlaneAppURLPath:         writeSettingForTest(t, directory, "central-plane-app-url", "https://company.example"),
		CentralPlaneProjectURLPath:     writeSettingForTest(t, directory, "central-plane-project-url", "http://127.0.0.1:54321"),
		CentralPlanePublishableKeyPath: writeSettingForTest(t, directory, "central-plane-publishable-key", "local-publishable"),
	}.withDefaults()

	if configuration.CentralPlaneAppURL != "https://company.example" {
		t.Fatalf("app url = %q", configuration.CentralPlaneAppURL)
	}
	if configuration.CentralPlaneProjectURL != "http://127.0.0.1:54321" {
		t.Fatalf("project url = %q", configuration.CentralPlaneProjectURL)
	}
	if configuration.CentralPlanePublishableKey != "local-publishable" {
		t.Fatalf("publishable key = %q", configuration.CentralPlanePublishableKey)
	}
}

func TestADeviceThatNamesNoRecordKeepsTheCompiledOne(t *testing.T) {
	stampCompiledCentralPlane(t, "https://compiled.example.test", "compiled-publishable")
	directory := t.TempDir()
	configuration := Configuration{
		CentralPlaneProjectURLPath:     filepath.Join(directory, "absent-project-url"),
		CentralPlanePublishableKeyPath: filepath.Join(directory, "absent-publishable-key"),
	}.withDefaults()

	if configuration.CentralPlaneProjectURL != "https://compiled.example.test" {
		t.Fatalf("project url = %q", configuration.CentralPlaneProjectURL)
	}
	if configuration.CentralPlanePublishableKey != "compiled-publishable" {
		t.Fatalf("publishable key = %q", configuration.CentralPlanePublishableKey)
	}
}

func stampCompiledCentralPlane(t *testing.T, projectURL string, publishableKey string) {
	t.Helper()
	previousProjectURL, previousPublishableKey := centralplane.DefaultProjectURL, centralplane.DefaultPublishableKey
	centralplane.DefaultProjectURL, centralplane.DefaultPublishableKey = projectURL, publishableKey
	t.Cleanup(func() {
		centralplane.DefaultProjectURL, centralplane.DefaultPublishableKey = previousProjectURL, previousPublishableKey
	})
}

// A flag is the operator saying it out loud, so it outranks a file the last
// setup happened to leave behind.
func TestWhatTheCommandLineNamesOutranksTheFiles(t *testing.T) {
	directory := t.TempDir()
	configuration := Configuration{
		CentralPlaneProjectURL:         "http://named-on-the-command-line",
		CentralPlaneProjectURLPath:     writeSettingForTest(t, directory, "central-plane-project-url", "http://named-in-a-file"),
		CentralPlanePublishableKeyPath: filepath.Join(directory, "absent"),
	}.withDefaults()

	if configuration.CentralPlaneProjectURL != "http://named-on-the-command-line" {
		t.Fatalf("project url = %q", configuration.CentralPlaneProjectURL)
	}
}
