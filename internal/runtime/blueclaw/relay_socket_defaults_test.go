package blueclaw

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// The relay is TypeScript and cannot read the layout, so it keeps the Linux
// paths as its defaults. They are a copy, and this is what keeps it one.
func TestTheRelaysSocketDefaultsAreTheLinuxLayouts(t *testing.T) {
	layout := LinuxCompanyHostLayout()
	for _, expected := range []struct {
		fileName string
		constant string
		path     string
	}{
		{"forward.ts", "defaultAdmindSocketPath", layout.AdmindSocketPath()},
		{"acp-session.ts", "defaultBlueclawACPSocketPath", layout.ACPSocketPath()},
	} {
		source, errorValue := os.ReadFile(filepath.Join("..", "..", "..", "host", "relay", expected.fileName))
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		match := regexp.MustCompile(`export const ` + expected.constant + ` = '([^']+)';`).FindSubmatch(source)
		if match == nil {
			t.Fatalf("host/relay/%s declares no %s, so this guard is reading the wrong source", expected.fileName, expected.constant)
		}
		if string(match[1]) != expected.path {
			t.Errorf("the relay defaults %s to %s and the Linux layout puts it at %s", expected.constant, match[1], expected.path)
		}
	}
}
