package devplane

import (
	"strings"
	"testing"
)

func joinedPlanArguments(plans []CommandPlan) string {
	joined := []string{}
	for _, plan := range plans {
		joined = append(joined, plan.Name+" "+strings.Join(plan.Arguments, " "))
	}
	return strings.Join(joined, "\n")
}

func TestThePlaneRunsItsTestsInALinuxGuestAgainstTheLocalRecord(t *testing.T) {
	service, errorValue := NewService(Options{
		RepositoryRootPath: "/repository",
		ExecutablePath:     "/repository/internkim",
		TestArguments:      []string{"-t", "the agent's directory"},
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	plans := joinedPlanArguments(service.planePlans())
	for _, expected := range []string{
		"tools/start-dev-plane-app", "tools/prepare-company-plane", "tools/prepare-container-kernel",
		"lab vm-up", "provision-blueclaw-dev-session.sh", "scenario-company-plane.sh",
		"'-t' 'the agent'\"'\"'s directory'",
	} {
		if !strings.Contains(plans, expected) {
			t.Fatalf("the plane's plans do not include %q:\n%s", expected, plans)
		}
	}
}

func TestEveryRunRemovesTheGuestItStarted(t *testing.T) {
	service, errorValue := NewService(Options{RepositoryRootPath: "/repository", ExecutablePath: "/repository/internkim"})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	removal := service.removeGuestCommand()
	if !strings.Contains(removal, "container rm '"+service.virtualMachineName+"'") {
		t.Fatalf("the run does not remove its own guest: %s", removal)
	}
	if !strings.HasPrefix(service.virtualMachineName, "internkim-dev-plane-") {
		t.Fatalf("the guest %q is not one the orphan reaper recognises", service.virtualMachineName)
	}
}
