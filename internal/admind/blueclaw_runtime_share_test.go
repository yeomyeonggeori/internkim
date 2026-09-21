package admind

import (
	"encoding/json"
	"testing"

	blueclawruntime "gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

func renderedDeviceRuntimeDocument(t *testing.T) string {
	t.Helper()
	document, errorValue := blueclawruntime.BlueclawRuntimeConfigDocumentWithOptions(blueclawruntime.RuntimeConfigOptions{
		DatabaseConnectionString: "postgres://internkim@postgres/tenant_01?sslmode=disable",
		WorkspaceRootPath:        "/workspace",
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return document
}

func databaseSectionOf(t *testing.T, document string) map[string]any {
	t.Helper()
	var decoded map[string]any
	if errorValue := json.Unmarshal([]byte(document), &decoded); errorValue != nil {
		t.Fatal(errorValue)
	}
	databaseSection, isPresent := decoded["database"].(map[string]any)
	if !isPresent {
		t.Fatalf("the document carries no database section: %s", document)
	}
	return databaseSection
}

func documentWithoutTheConnectionShare(t *testing.T, document string) string {
	t.Helper()
	var decoded map[string]any
	if errorValue := json.Unmarshal([]byte(document), &decoded); errorValue != nil {
		t.Fatal(errorValue)
	}
	databaseSection, isPresent := decoded["database"].(map[string]any)
	if !isPresent {
		t.Fatalf("the document carries no database section: %s", document)
	}
	if _, carriesShare := databaseSection[blueclawruntime.AgentDatabaseConnectionsField]; !carriesShare {
		t.Fatal("the renderer no longer writes the share, so removing it proves nothing")
	}
	delete(databaseSection, blueclawruntime.AgentDatabaseConnectionsField)
	older, errorValue := json.MarshalIndent(decoded, "", "  ")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return string(older) + "\n"
}

// Every device provisioned before the share existed holds a document without
// it, and the renderer never runs again on a device that is already installed.
// Only this refresh reaches them.
func TestADeviceDocumentWrittenBeforeTheShareExistedGainsItOnRefresh(t *testing.T) {
	older := documentWithoutTheConnectionShare(t, renderedDeviceRuntimeDocument(t))

	refreshed, errorValue := refreshedBlueclawRuntimeConfiguration(older, blueclawruntime.CurrentCapabilityContract())
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	share, carriesShare := databaseSectionOf(t, refreshed)[blueclawruntime.AgentDatabaseConnectionsField]
	if !carriesShare {
		t.Fatalf("the refresh left the document without database.%s, so an installed device keeps asking the server for everything it allows", blueclawruntime.AgentDatabaseConnectionsField)
	}
	if share != float64(blueclawruntime.AgentDatabaseConnections) {
		t.Fatalf("the refresh granted the agent %v connections against the %d the budget reserves for it", share, blueclawruntime.AgentDatabaseConnections)
	}
}

// The refresh only runs on a document the staleness check calls stale, so a
// device missing the share has to read as stale or the stamp never reaches it.
func TestADeviceDocumentMissingTheShareReadsAsStale(t *testing.T) {
	older := documentWithoutTheConnectionShare(t, renderedDeviceRuntimeDocument(t))

	refreshed, errorValue := refreshedBlueclawRuntimeConfiguration(older, blueclawruntime.CurrentCapabilityContract())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if refreshed == older {
		t.Fatal("a document missing the share refreshes to itself, so the reconcile would call it current and never restamp it")
	}
}

// A stamp that rewrites a document it has already stamped makes every reconcile
// pass restamp and re-deliver, which is a release loop rather than a repair.
func TestRefreshingATwiceStampedDocumentChangesNothing(t *testing.T) {
	contract := blueclawruntime.CurrentCapabilityContract()

	once, errorValue := refreshedBlueclawRuntimeConfiguration(renderedDeviceRuntimeDocument(t), contract)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	twice, errorValue := refreshedBlueclawRuntimeConfiguration(once, contract)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if once != twice {
		t.Fatal("refreshing an already refreshed document changed it, so every reconcile would restamp and re-deliver")
	}
}
