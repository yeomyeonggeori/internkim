package localfleet

import (
	"strings"
	"testing"
)

func TestCompanyPlaneRunsInManagedLinuxVirtualMachine(t *testing.T) {
	service, errorValue := NewService(Options{
		RepositoryRootPath: "/repo", ExecutablePath: "/repo/internkim",
		RunID: "plane-test", IsEphemeral: true, CompanyAppPort: 5197,
		ScenarioArguments: []string{"-t", "the agent's directory"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	plans := joinedPlanArguments(service.companyPlaneScenarioPlans())
	for _, expected := range []string{
		"prepare-company-plane", "vm-up", "provision-blueclaw-dev-session.sh",
		"scenario-company-plane.sh", "'5197'", "'-t'", "'the agent'\"'\"'s directory'",
	} {
		if !strings.Contains(plans, expected) {
			t.Fatalf("missing %q in company plane plans: %s", expected, plans)
		}
	}
	for _, name := range ScenarioNames() {
		if name == "company-plane" {
			t.Fatal("the company plane is run by dev plane and is not a fleet scenario")
		}
	}
	cleanup := joinedPlanArguments(service.ephemeralCleanupPlans())
	if !strings.Contains(cleanup, "'container' rm 'internkim-e2e-plane-test'") {
		t.Fatalf("disposable company plane has no VM cleanup: %s", cleanup)
	}
}
