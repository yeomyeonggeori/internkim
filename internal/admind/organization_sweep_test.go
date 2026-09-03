package admind

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type companyHoldingProfiles struct {
	server  *httptest.Server
	written []recordedProfileWrite
}

type recordedProfileWrite struct {
	Email    string
	JobTitle string
	TeamID   string
}

// The carry writes a profile through person_update, and person_update names a
// team by id, so the teams the profiles name are settled through team_list and
// team_add first. This answers all three.
func startCompanyForOrganizationCarry(t *testing.T) *companyHoldingProfiles {
	t.Helper()
	company := &companyHoldingProfiles{}
	teams := []map[string]any{}
	company.server = httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.Header().Set("Content-Type", "application/json")
		switch {
		case request.URL.Path == "/api/agent/session":
			_, _ = responseWriter.Write([]byte(`{"memberID":"member-admin","accessToken":"token","expiresAt":4102444800}`))
		case request.URL.Path == "/api/agent/member":
			_, _ = responseWriter.Write([]byte(`{"members":[{"memberID":"member-1","email":"colleague@example.com","status":"active"}]}`))
		case strings.HasPrefix(request.URL.Path, "/api/v1/tools/"):
			toolName := strings.TrimSuffix(strings.TrimPrefix(request.URL.Path, "/api/v1/tools/"), "/invoke")
			var payload struct {
				Input map[string]any `json:"input"`
			}
			_ = json.NewDecoder(request.Body).Decode(&payload)
			switch toolName {
			case "team_list":
				writeToolAnswer(responseWriter, toolName, map[string]any{"teams": teams})
			case "team_add":
				made := map[string]any{
					"teamID":       "team-" + stringInput(payload.Input, "name"),
					"name":         payload.Input["name"],
					"parentTeamID": "",
					"position":     0,
				}
				teams = append(teams, made)
				writeToolAnswer(responseWriter, toolName, made)
			case "person_update":
				company.written = append(company.written, recordedProfileWrite{
					Email:    stringInput(payload.Input, "personHint"),
					JobTitle: stringInput(payload.Input, "jobTitle"),
					TeamID:   stringInput(payload.Input, "teamHint"),
				})
				writeToolAnswer(responseWriter, toolName, map[string]any{"personID": "member-1"})
			default:
				http.Error(responseWriter, "the company answers no tool called "+toolName, http.StatusNotFound)
			}
		default:
			_, _ = responseWriter.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(company.server.Close)
	return company
}

func writeToolAnswer(responseWriter http.ResponseWriter, toolName string, result map[string]any) {
	document, _ := json.Marshal(map[string]any{"tool": toolName, "result": result})
	_, _ = responseWriter.Write(document)
}

func stringInput(input map[string]any, field string) string {
	value, _ := input[field].(string)
	return value
}

func newOrganizationSweepTestService(t *testing.T, planeURL string) *Service {
	t.Helper()
	stateDirectory := t.TempDir()
	configuration := Configuration{
		StateDirectory:        stateDirectory,
		ClaimedAdminEmailPath: writeTestFile(t, "admin@example.com"),
	}
	if planeURL != "" {
		configuration.CentralPlaneAppURL = planeURL
		configuration.CentralPlaneProjectURL = planeURL
		configuration.CentralPlanePublishableKey = "publishable"
		configuration.CentralPlaneAgentKeyPath = writeAgentKeyForTest(t, "agent-key")
	}
	return NewService(configuration)
}

func seedRetiredOrganizationTables(t *testing.T, databasePath string) *sql.DB {
	t.Helper()
	if errorValue := os.MkdirAll(filepath.Dir(databasePath), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	database, errorValue := sql.Open("sqlite", sqliteDatabaseDSN(databasePath))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, statement := range []string{
		`CREATE TABLE organization_profiles (profile_key TEXT PRIMARY KEY, user_id TEXT NOT NULL, email TEXT NOT NULL,
			job_title TEXT NOT NULL, group_id TEXT NOT NULL, phone_number TEXT NOT NULL, hire_date TEXT NOT NULL,
			supervisor_id TEXT NOT NULL, status TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		"CREATE TABLE organization_groups (id TEXT PRIMARY KEY, name TEXT NOT NULL, parent_id TEXT NOT NULL, position INTEGER NOT NULL)",
		"CREATE TABLE organization_metadata (key TEXT PRIMARY KEY, value TEXT NOT NULL)",
		"CREATE TABLE organization_people_cache_entries (cache_key TEXT PRIMARY KEY)",
		"CREATE TABLE organization_people_cache_states (cache_key TEXT PRIMARY KEY)",
	} {
		if _, errorValue := database.ExecContext(context.Background(), statement); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	if _, errorValue := database.ExecContext(context.Background(),
		"INSERT INTO organization_people_cache_entries (cache_key) VALUES ('list')"); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := database.ExecContext(context.Background(),
		"INSERT INTO organization_groups (id, name, parent_id, position) VALUES ('team-sales', '영업팀', '', 0)"); errorValue != nil {
		t.Fatal(errorValue)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func seedDeviceOrganizationProfile(t *testing.T, database *sql.DB, profileKey string, email string, jobTitle string, groupID string) {
	t.Helper()
	if _, errorValue := database.ExecContext(context.Background(), `
		INSERT INTO organization_profiles (profile_key, user_id, email, job_title, group_id, phone_number, hire_date, supervisor_id, status, updated_at)
		VALUES (?, '', ?, ?, ?, '', '', '', 'active', '2026-07-06T00:00:00Z')`,
		profileKey, email, jobTitle, groupID); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func countOrganizationTablesNamed(t *testing.T, database *sql.DB, tableName string) int {
	t.Helper()
	var tableCount int
	if errorValue := database.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = ?", tableName).Scan(&tableCount); errorValue != nil {
		t.Fatal(errorValue)
	}
	return tableCount
}

func openOrganizationStoreForTest(t *testing.T, service *Service) *sql.DB {
	t.Helper()
	database, errorValue := service.openOrganizationDatabase(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	t.Cleanup(func() { database.Close() })
	return database
}

func TestOrganizationSweepDropsTheDerivedTablesWhatever(t *testing.T) {
	service := newOrganizationSweepTestService(t, "")
	seedRetiredOrganizationTables(t, service.stateDatabasePath())
	seedDeviceOrganizationProfile(t, openOrganizationStoreForTest(t, service), "email:colleague@example.com", "colleague@example.com", "편집장", "")

	service.sweepTheOrganizationTheCompanyNowHolds(context.Background())

	database := openOrganizationStoreForTest(t, service)
	if countOrganizationTablesNamed(t, database, "organization_people_cache_entries") != 0 {
		t.Fatal("a cache is not a record of anything, and it stayed")
	}
	if countOrganizationTablesNamed(t, database, "organization_profiles") == 0 {
		t.Fatal("a device that names no company let go of profiles nobody else holds")
	}
}

// A carried row is still in the local table, so a sweep that only counted rows
// would keep the store forever and the carry would never finish anything.
func TestOrganizationSweepDropsTheStoreOnceTheCarryHasFinished(t *testing.T) {
	company := startCompanyForOrganizationCarry(t)
	service := newOrganizationSweepTestService(t, company.server.URL)
	seedRetiredOrganizationTables(t, service.stateDatabasePath())
	seedDeviceOrganizationProfile(t, openOrganizationStoreForTest(t, service), "email:colleague@example.com", "colleague@example.com", "편집장", "team-sales")

	report, errorValue := service.carryTheOrganizationIntoTheRecord(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if report.Profiles != 1 {
		t.Fatalf("report = %+v", report)
	}
	if len(company.written) != 1 || company.written[0].Email != "colleague@example.com" || company.written[0].TeamID != "team-영업팀" {
		t.Fatalf("the record was given %+v", company.written)
	}

	service.sweepTheOrganizationTheCompanyNowHolds(context.Background())

	database := openOrganizationStoreForTest(t, service)
	if countOrganizationTablesNamed(t, database, "organization_profiles") != 0 {
		t.Fatal("the store stayed after the record took everything in it")
	}
	if countOrganizationTablesNamed(t, database, "organization_carried_rows") != 0 {
		t.Fatal("the memory of a carry outlived what it described")
	}
}

func TestOrganizationCarryRefusesWithoutAClaimedAdministrator(t *testing.T) {
	company := startCompanyForOrganizationCarry(t)
	service := newOrganizationSweepTestService(t, company.server.URL)
	service.Configuration.ClaimedAdminEmailPath = writeTestFile(t, "")
	seedRetiredOrganizationTables(t, service.stateDatabasePath())
	seedDeviceOrganizationProfile(t, openOrganizationStoreForTest(t, service), "email:colleague@example.com", "colleague@example.com", "편집장", "")

	if _, errorValue := service.carryTheOrganizationIntoTheRecord(context.Background()); errorValue == nil {
		t.Fatal("a profile is an administrator's to record, and the carry wrote one as nobody")
	}
}

// The store wrote a row for everybody the directory named, described or not. A
// row saying nothing about a person is not something the record is missing.
func TestOrganizationSweepIgnoresARowThatDescribesNobody(t *testing.T) {
	company := startCompanyForOrganizationCarry(t)
	service := newOrganizationSweepTestService(t, company.server.URL)
	seedRetiredOrganizationTables(t, service.stateDatabasePath())
	seedDeviceOrganizationProfile(t, openOrganizationStoreForTest(t, service), "email:blank@example.com", "blank@example.com", "", "")

	service.sweepTheOrganizationTheCompanyNowHolds(context.Background())

	database := openOrganizationStoreForTest(t, service)
	if countOrganizationTablesNamed(t, database, "organization_profiles") != 0 {
		t.Fatal("the store stayed for a row that says nothing about anybody")
	}
	if len(company.written) != 0 {
		t.Fatalf("an empty row was carried: %+v", company.written)
	}
}
