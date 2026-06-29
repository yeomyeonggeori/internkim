package admind

import (
	"strings"
	"testing"
)

func TestReleaseServiceStateIsDisabled(t *testing.T) {
	for _, state := range []string{"disabled", "masked", "masked-runtime", " disabled\n"} {
		if !releaseServiceStateIsDisabled(state) {
			t.Fatalf("expected %q to count as disabled", state)
		}
	}
	for _, state := range []string{"enabled", "static", "alias", "", "indirect"} {
		if releaseServiceStateIsDisabled(state) {
			t.Fatalf("expected %q not to count as disabled", state)
		}
	}
}

func TestTenantServiceIsDisabledNeverSkipsDefaultService(t *testing.T) {
	service := &Service{}
	if service.tenantServiceIsDisabled(nil, "blueclaw.service") {
		t.Fatal("the default blueclaw.service must never be treated as a disabled tenant")
	}
	if service.tenantServiceIsDisabled(nil, "internkim-admind.service") {
		t.Fatal("a non-tenant service must never be treated as a disabled tenant")
	}
}

func TestRemoveTenantPilotsActionAllowedAndScoped(t *testing.T) {
	if !isAllowedSSHRecoveryAction("remove-tenant-pilots") {
		t.Fatal("remove-tenant-pilots must be an allowed server recovery action")
	}
	command := removeTenantPilotsCommand()
	for _, fragment := range []string{"internkim-tenant-*pilot*", "systemctl daemon-reload", "/srv/internkim/tenants/pilot-*", "rm -rf"} {
		if !strings.Contains(command, fragment) {
			t.Fatalf("remove command must contain %q, got:\n%s", fragment, command)
		}
	}
	if strings.Contains(command, "rm -rf /srv/internkim/tenants ") || strings.Contains(command, "rm -rf /\n") {
		t.Fatal("remove command must be scoped to pilot dirs, not the whole tenants tree")
	}
}
