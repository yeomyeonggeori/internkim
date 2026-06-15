package admind

import "testing"

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
