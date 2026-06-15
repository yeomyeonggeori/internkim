package cli

import "testing"

func TestRemoveTenantPilotsCLIActionAllowed(t *testing.T) {
	if !isAllowedCLIRecoveryAction("remove-tenant-pilots") {
		t.Fatal("remove-tenant-pilots must be an allowed CLI recovery action")
	}
	if isAllowedCLIRecoveryAction("rm-everything") {
		t.Fatal("unknown actions must be rejected")
	}
}
