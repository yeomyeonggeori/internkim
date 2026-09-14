package blueclaw

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const companyImageDockerfilePath = "../../../host/Dockerfile"

func TestTheCompanyRuntimeTemplateNamesTheHelperTheDeviceNames(t *testing.T) {
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
	if runtimeConfiguration.Terminal.POSIXHelperPath != BlueclawPOSIXHelperPath {
		t.Fatalf("the company template names helper %q and the device names %q; without the helper blueclaw refuses every shell and file tool", runtimeConfiguration.Terminal.POSIXHelperPath, BlueclawPOSIXHelperPath)
	}
}

func TestTheCompanyImageInstallsTheHelperWhereTheTemplateNamesIt(t *testing.T) {
	dockerfile, errorValue := os.ReadFile(filepath.Clean(companyImageDockerfilePath))
	if errorValue != nil {
		t.Fatalf("expected the company image Dockerfile: %v", errorValue)
	}
	if !strings.Contains(string(dockerfile), "chmod 4755 "+BlueclawPOSIXHelperPath) {
		t.Fatalf("host/Dockerfile does not install a setuid helper at %s, so the runtime names a helper the image does not carry", BlueclawPOSIXHelperPath)
	}
}
