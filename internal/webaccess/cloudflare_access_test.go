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

func TestAdminAccessApplicationDomainsProtectOnlyInternKimPaths(t *testing.T) {
	document := readRepositoryFile(t, "web/src/lib/cloudflare.ts")
	body := functionBody(t, document, "export function adminAccessApplicationDomains")
	specBody := functionBody(t, document, "function adminAccessApplicationSpecs")
	requiredFragments := []string{
		"${hostname}/admin*",
		"${hostname}/_app/*",
		"${hostname}/flow*",
		"${hostname}/mail*",
		"${hostname}/calendar",
		"${hostname}/calendar/",
		"${hostname}/calendar/api/*",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(specBody, fragment) {
			t.Fatalf("admin Access domains must include %q", fragment)
		}
	}
	disallowedFragments := []string{
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
		if strings.Contains(specBody, fragment) {
			t.Fatalf("admin Access domains must not protect Mattermost or token-auth path %q", fragment)
		}
	}
	if !strings.Contains(body, "adminAccessApplicationSpecs(env, fleetId).flatMap((spec) => spec.domains)") {
		t.Fatalf("admin Access domains must be derived from the split application specs")
	}
}

func TestAdminAccessApplicationsAreSplitByDestinationLimit(t *testing.T) {
	document := readRepositoryFile(t, "web/src/lib/cloudflare.ts")
	body := functionBody(t, document, "export async function ensureAdminAccessApplications")
	requiredFragments := []string{
		"await ensureCalendarClientBypassApplication(env, fleetId)",
		"for (const spec of adminAccessApplicationSpecs(env, fleetId))",
		"ensureAdminAccessApplication(env, fleetId, identityProviderId, spec)",
		"syncAccessPolicyEmails(env, applicationID, normalizeRequiredAccessEmails(adminEmails, 'Cloudflare admin access'))",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(body, fragment) {
			t.Fatalf("admin Access sync must include %q", fragment)
		}
	}
	if strings.Contains(document, "self_hosted_domains: adminAccessApplicationDomains(env, fleetId)") {
		t.Fatalf("admin Access apps must not put every protected path into one Cloudflare app")
	}
}

func TestCalendarClientPathsBypassCloudflareAccess(t *testing.T) {
	document := readRepositoryFile(t, "web/src/lib/cloudflare.ts")
	body := functionBody(t, document, "function calendarClientBypassApplicationBody")
	requiredFragments := []string{
		"${hostname}/calendar/ics/*",
		"${hostname}/calendar/dav/*",
		"${hostname}/.well-known/caldav",
	}
	for _, fragment := range requiredFragments {
		if !strings.Contains(body, fragment) {
			t.Fatalf("calendar client bypass app must include %q", fragment)
		}
	}
	if !strings.Contains(document, "await ensureBypassPolicy(env, applicationId, 'calendar-clients')") {
		t.Fatalf("calendar client app must use a Cloudflare Access bypass policy")
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
