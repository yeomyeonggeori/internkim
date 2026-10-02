package blueclaw

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const companyRuntimeRendererPath = "../../../tools/render-company-runtime"

// The device and the package install the helper in different places, because a
// Debian package may not write /usr/local. The template therefore names neither:
// it carries the hole, the renderer's default fills it with where the device
// keeps it, and the packaged prepare script fills it with what dpkg installs.
// A runtime document naming a helper that is not there costs every shell and
// file tool, and reports ok while doing it.
func TestTheRenderedRuntimeNamesAHelperWhicheverInstalledIt(t *testing.T) {
	document, errorValue := os.ReadFile(filepath.Clean(companyRuntimeTemplatePath))
	if errorValue != nil {
		t.Fatalf("expected the company runtime template: %v", errorValue)
	}
	runtimeConfiguration := struct {
		Terminal struct {
			POSIXHelperPath string `json:"posixHelperPath"`
		} `json:"terminal"`
	}{}
	if errorValue := json.Unmarshal(document, &runtimeConfiguration); errorValue != nil {
		t.Fatalf("expected a runtime configuration document: %v", errorValue)
	}
	if runtimeConfiguration.Terminal.POSIXHelperPath != "${POSIX_HELPER_PATH}" {
		t.Fatalf("the template names helper %q rather than leaving the hole the renderer fills", runtimeConfiguration.Terminal.POSIXHelperPath)
	}

	renderer, errorValue := os.ReadFile(filepath.Clean(companyRuntimeRendererPath))
	if errorValue != nil {
		t.Fatalf("expected the company runtime renderer: %v", errorValue)
	}
	if !strings.Contains(string(renderer), `: "${POSIX_HELPER_PATH:=`+BlueclawPOSIXHelperPath+`}"`) {
		t.Fatalf("render-company-runtime does not default the helper to %s, which is where the device keeps it", BlueclawPOSIXHelperPath)
	}
	if !strings.Contains(CompanyHostPrepareScript(), "POSIX_HELPER_PATH="+LinuxCompanyHostLayout().POSIXHelperPath()) {
		t.Fatalf("the packaged prepare script does not render the helper as %s, which is what the package installs", LinuxCompanyHostLayout().POSIXHelperPath())
	}
}
