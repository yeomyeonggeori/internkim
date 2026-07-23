package cli

import (
	"strings"
	"testing"
)

func TestSelectedReleaseComponentNamesParsesCommaList(t *testing.T) {
	components, errorValue := selectedReleaseComponentNames([]string{"--components", "capabilityd,admind"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !components["capabilityd"] || !components["admind"] {
		t.Fatalf("components = %+v", components)
	}
	if components["skills"] {
		t.Fatalf("unexpected skills component in %+v", components)
	}
}

func TestCoupleBlueclawWithSkillsAddsSkillsForPayload(t *testing.T) {
	coupled := coupleBlueclawWithSkills(map[string]bool{"blueclawPayload": true})
	if !coupled["blueclawPayload"] || !coupled["skills"] {
		t.Fatalf("expected payload deploy to carry skills, got %+v", coupled)
	}
}

func TestCoupleBlueclawWithSkillsAddsProtocolComponentsForLLMD(t *testing.T) {
	coupled := coupleBlueclawWithSkills(map[string]bool{"blueclawLLMD": true})
	for _, componentName := range []string{"admind", "blueclawLLMD", "blueclawPayload", "capabilityd", "skills"} {
		if !coupled[componentName] {
			t.Fatalf("expected LLMD deploy to carry %s, got %+v", componentName, coupled)
		}
	}
}

func TestCoupleBlueclawWithSkillsLeavesOtherComponentsAlone(t *testing.T) {
	coupled := coupleBlueclawWithSkills(map[string]bool{"admind": true, "web": true})
	if coupled["skills"] {
		t.Fatalf("expected no skills coupling without the payload, got %+v", coupled)
	}
}

func TestSelectedReleaseComponentNamesNormalizesLegacyWebNames(t *testing.T) {
	components, errorValue := selectedReleaseComponentNames([]string{"--components", "adminWeb,admin-web,web"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(components) != 1 || !components["web"] {
		t.Fatalf("components = %+v", components)
	}
}

func TestSelectedReleaseComponentNamesAllowsAllByDefault(t *testing.T) {
	components, errorValue := selectedReleaseComponentNames(nil)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if components != nil {
		t.Fatalf("components = %+v, want nil for all components", components)
	}
}

func TestLegacySSHDeploySetupStepNamesMapsPayloadDirectly(t *testing.T) {
	stepNames, hasSelectedSteps, errorValue := legacySSHDeploySetupStepNames([]string{"--components", "blueclawPayload"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !hasSelectedSteps {
		t.Fatal("expected selected setup steps")
	}
	if strings.Join(stepNames, ",") != "blueclaw-payload-direct" {
		t.Fatalf("stepNames = %v", stepNames)
	}
}

func TestLegacySSHDeploySetupStepNamesKeepsStableOrder(t *testing.T) {
	stepNames, hasSelectedSteps, errorValue := legacySSHDeploySetupStepNames([]string{"--components", "blueclawPayload,web,skills"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !hasSelectedSteps {
		t.Fatal("expected selected setup steps")
	}
	if strings.Join(stepNames, ",") != "web,skills,blueclaw-payload-direct" {
		t.Fatalf("stepNames = %v", stepNames)
	}
}

func TestLegacySSHDeploySetupStepNamesRejectsUnsupportedComponents(t *testing.T) {
	_, hasSelectedSteps, errorValue := legacySSHDeploySetupStepNames([]string{"--components", "internkim"})
	if errorValue == nil {
		t.Fatal("expected unsupported component error")
	}
	if !hasSelectedSteps {
		t.Fatal("expected selected setup steps")
	}
	if !strings.Contains(errorValue.Error(), "internkim") {
		t.Fatalf("error = %v", errorValue)
	}
}

func TestDeployUsageTextContainsComponentNames(t *testing.T) {
	usage := deployUsageText()
	for _, componentName := range []string{"admind", "blueclawPayload", "blueclawLLMD", "capabilityd", "internkim", "mattermostPlugins", "skills", "web"} {
		if !strings.Contains(usage, componentName) {
			t.Errorf("deployUsageText() missing component name %q", componentName)
		}
	}
	if !strings.Contains(usage, "--components") {
		t.Error("deployUsageText() missing --components flag")
	}
}

func TestValidateDeployArgumentsAcceptsHelpFlag(t *testing.T) {
	if errorValue := validateDeployArguments([]string{"--help"}); errorValue != nil {
		t.Fatalf("validateDeployArguments(--help) returned error: %v", errorValue)
	}
	if errorValue := validateDeployArguments([]string{"-h"}); errorValue != nil {
		t.Fatalf("validateDeployArguments(-h) returned error: %v", errorValue)
	}
}

func TestValidateDeployArgumentsAcceptsKnownFlags(t *testing.T) {
	knownCases := [][]string{
		{},
		{"--components", "admind,web"},
		{"--components=admind"},
		{"--release", "r1"},
		{"--channel", "stable"},
		{"--legacy-ssh"},
		{"--node", "abc"},
	}
	for _, arguments := range knownCases {
		if errorValue := validateDeployArguments(arguments); errorValue != nil {
			t.Fatalf("validateDeployArguments(%v) returned unexpected error: %v", arguments, errorValue)
		}
	}
}

func TestValidateDeployArgumentsRejectsUnknownFlags(t *testing.T) {
	unknownCases := [][]string{
		{"--foo"},
		{"--dry-run"},
		{"--unknown=value"},
		{"--components", "admind", "--typo"},
	}
	for _, arguments := range unknownCases {
		if errorValue := validateDeployArguments(arguments); errorValue == nil {
			t.Fatalf("validateDeployArguments(%v) expected error, got nil", arguments)
		}
	}
}
