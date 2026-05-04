package blueclaw

import (
	"strings"
	"testing"
)

func TestUsersSyncScriptReadsCanonicalAdminEmailWithLegacyFallback(t *testing.T) {
	script := InternKimUsersSyncScript()

	for _, fragment := range []string{
		"/root/.internkim/config/admin-email",
		"/root/.internkim/admin-email",
	} {
		if !strings.Contains(script, fragment) {
			t.Fatalf("expected users sync script to include %q", fragment)
		}
	}
}
