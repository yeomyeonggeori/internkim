package tenantruntime

import (
	"strings"
	"testing"
)

func TestGenerateTenantCredentialsIssuesUnbrandedAdminPassword(t *testing.T) {
	credentials, errorValue := GenerateTenantCredentials()
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	if credentials.AdminUsername != TenantInitialAdminUsername {
		t.Fatalf("admin username = %q", credentials.AdminUsername)
	}
	if strings.HasPrefix(credentials.AdminPassword, "InternKim") {
		t.Fatalf("admin password has predictable prefix: %q", credentials.AdminPassword)
	}
	if len(credentials.AdminPassword) < 36 {
		t.Fatalf("admin password is too short: %q", credentials.AdminPassword)
	}
}
