package webaccess

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFleetAccessDoesNotCreateRootAccessApplication(t *testing.T) {
	document := readRepositoryFile(t, "web/src/lib/fleet-access.ts")
	disallowedFragments := []string{
		"createAccessApplication",
		"updateAccessApplicationLoginMethod",
		"userEmails",
	}
	for _, fragment := range disallowedFragments {
		if strings.Contains(document, fragment) {
			t.Fatalf("fleet access sync must not create or update root Access app via %q", fragment)
		}
	}
	requiredFragments := []string{
		"deleteRootAccessApplication(env, fleetID, device.access_app_id)",
		"access_app_id: undefined",
		"access_policy_id: undefined",
		"ensureAdminAccessApplications(env, fleetID, identityProviderID, adminAccessEmails)",
		"syncSSHAccessPolicyEmails(env, applicationID, adminAccessEmails)",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(document, fragment) {
			t.Fatalf("fleet access sync must include %q", fragment)
		}
	}
}

func TestAdminAccessApplicationDomainsProtectOnlyAdmin(t *testing.T) {
	document := readRepositoryFile(t, "web/src/lib/cloudflare.ts")
	body := functionBody(t, document, "export function adminAccessApplicationDomains")
	requiredFragments := []string{
		"${hostname}/admin*",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(body, fragment) {
			t.Fatalf("admin Access domains must include %q", fragment)
		}
	}
	disallowedFragments := []string{
		"/_app",
		"/flow",
		"/mail",
		"/calendar",
		"/api/v4",
		"/websocket",
		"/plugins",
		"/static",
		"/_internkim/companion",
		"/_internkim/runtime",
		"/calendar/ics",
		"/calendar/dav",
		"/.well-known/caldav",
	}
	for _, fragment := range disallowedFragments {
		if strings.Contains(body, fragment) {
			t.Fatalf("admin Access domains must not protect non-admin path %q", fragment)
		}
	}
}

func TestAdminAccessApplicationsDeleteLegacyNonAdminApps(t *testing.T) {
	document := readRepositoryFile(t, "web/src/lib/cloudflare.ts")
	body := functionBody(t, document, "export async function ensureAdminAccessApplications")
	requiredFragments := []string{
		"await deleteLegacyNonAdminAccessApplications(env, fleetId)",
		"const applicationID = await ensureAdminAccessApplication(env, fleetId, identityProviderId)",
		"syncAccessPolicyEmails(env, applicationID, normalizeRequiredAccessEmails(adminEmails, 'Cloudflare admin access'))",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(body, fragment) {
			t.Fatalf("admin Access sync must include %q", fragment)
		}
	}

	cleanupBody := functionBody(t, document, "async function deleteLegacyNonAdminAccessApplications")
	legacyDomains := []string{
		"'/_app/*'",
		"'/flow*'",
		"'/mail*'",
		"'/calendar'",
		"'/calendar/ics/*'",
		"'/_internkim/admin/*'",
	}
	for _, domain := range legacyDomains {
		if !strings.Contains(cleanupBody, domain) {
			t.Fatalf("legacy non-admin Access cleanup must include %q", domain)
		}
	}
	if strings.Contains(document, "ensureCalendarClientBypassApplication") {
		t.Fatalf("calendar must not create a Cloudflare Access bypass app")
	}
}

func TestRootAccessApplicationDeletionMatchesLegacyIDAndApexDomain(t *testing.T) {
	document := readRepositoryFile(t, "web/src/lib/cloudflare.ts")
	body := functionBody(t, document, "export async function deleteRootAccessApplication")
	requiredFragments := []string{
		"application.id === applicationId",
		"accessApplicationDomain(application) === hostname",
		"deleteAccessApplication(env, id)",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(body, fragment) {
			t.Fatalf("root Access deletion must include %q", fragment)
		}
	}
}

func readRepositoryFile(t *testing.T, path string) string {
	t.Helper()
	document, errorValue := os.ReadFile(filepath.Join("..", "..", path))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return string(document)
}

func functionBody(t *testing.T, document string, marker string) string {
	t.Helper()
	start := strings.Index(document, marker)
	if start < 0 {
		t.Fatalf("missing function marker %q", marker)
	}
	end := strings.Index(document[start:], "\n}\n")
	if end < 0 {
		t.Fatalf("missing function end for %q", marker)
	}
	return document[start : start+end]
}
