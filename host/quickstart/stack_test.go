package quickstart

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type composeFile struct {
	Services map[string]struct {
		Image string `yaml:"image"`
	} `yaml:"services"`
}

// selfBuiltImageVariables names compose variables that hold an image this
// repository builds and tags itself (see Makefile's build-host target),
// rather than one pulled from a third-party registry. No digest exists for
// them before the build runs, so they are outside this test's guarantee.
var selfBuiltImageVariables = map[string]bool{
	"HOST_IMAGE": true,
}

// knownFloatingImages lists services whose image is not yet pinned to a
// digest, with the reason it stays that way for now. Each is a deliberate,
// reviewable exception, not a silent skip: dropping an entry re-enables the
// guarantee below for that service, and adding one requires stating why.
var knownFloatingImages = map[string]string{
	"postgres": "official library/postgres image; reachable today, pinning tracked as follow-up outside this change",
	"redis":    "official library/redis image; reachable today, pinning tracked as follow-up outside this change",
}

func TestEveryQuickstartImageIsPinnedToADigest(t *testing.T) {
	var compose composeFile
	if err := yaml.Unmarshal(ComposeFile, &compose); err != nil {
		t.Fatalf("parse compose.yaml: %v", err)
	}
	if len(compose.Services) == 0 {
		t.Fatal("compose.yaml declared no services; the parser or the fixture is broken")
	}

	resolvedVariables := map[string]string{
		"BUZZ_IMAGE": MessengerImage(),
	}

	for serviceName, service := range compose.Services {
		image := resolveComposeVariable(t, serviceName, service.Image, resolvedVariables)
		if image == "" {
			continue
		}
		if strings.Contains(image, "@sha256:") {
			continue
		}
		if reason, floats := knownFloatingImages[serviceName]; floats {
			t.Logf("service %q image %q is intentionally left unpinned: %s", serviceName, image, reason)
			continue
		}
		t.Errorf("service %q image %q floats: it names no sha256 digest, so the registry can silently swap what it serves (see host/quickstart/buzz-image for the pinned name:tag@sha256:digest pattern)", serviceName, image)
	}
}

func resolveComposeVariable(t *testing.T, serviceName, image string, resolved map[string]string) string {
	t.Helper()

	if !strings.HasPrefix(image, "${") || !strings.HasSuffix(image, "}") {
		return image
	}

	variable := strings.TrimSuffix(strings.TrimPrefix(image, "${"), "}")
	if selfBuiltImageVariables[variable] {
		return ""
	}

	resolvedImage, known := resolved[variable]
	if !known {
		t.Fatalf("service %q references variable %q, which this test does not know how to resolve; add it to resolvedVariables or selfBuiltImageVariables in host/quickstart/stack_test.go", serviceName, variable)
	}
	return resolvedImage
}
