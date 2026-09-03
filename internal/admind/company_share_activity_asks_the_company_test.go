package admind

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCompanyShareTeamActivityReadsTheBoardAsTheClaimedAdmin(t *testing.T) {
	service := newCompanyShareCompanyService(t)
	company := startCompanyHoldingOneTask(t, companyHeldTaskID, "completed")
	useCompanyForTest(service, company.URL)
	writeClaimedAdminEmailForTest(t, service, "admin@example.com")

	activity, errorValue := service.buildCompanyShareTeamActivity(t.Context(), time.Date(2026, time.July, 14, 12, 0, 0, 0, time.UTC))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if activity.WorkTotal != 1 {
		t.Fatalf("work total = %d, want the one board task: %#v", activity.WorkTotal, activity)
	}
	if len(activity.RecentWork) != 1 || activity.RecentWork[0].Title != "회사 업무" || activity.RecentWork[0].Status != "completed" {
		t.Fatalf("unexpected recent work: %#v", activity.RecentWork)
	}
	if activity.RecentWork[0].Date != "2026-07-06" {
		t.Fatalf("board task date = %q, want the updated day: %#v", activity.RecentWork[0].Date, activity.RecentWork)
	}
}

func TestCompanyShareTeamActivityPublishesNoWorkWithoutAClaimedAdmin(t *testing.T) {
	service := newCompanyShareCompanyService(t)
	company := startCompanyHoldingOneTask(t, companyHeldTaskID, "completed")
	useCompanyForTest(service, company.URL)

	activity, errorValue := service.buildCompanyShareTeamActivity(t.Context(), time.Date(2026, time.July, 14, 12, 0, 0, 0, time.UTC))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if activity.WorkTotal != 0 || len(activity.RecentWork) != 0 {
		t.Fatalf("without a claimed admin the work view must be empty, not local rows: %#v", activity)
	}
}

func newCompanyShareCompanyService(t *testing.T) *Service {
	t.Helper()
	temporaryDirectory := t.TempDir()
	service := NewService(Configuration{
		StateDirectory:         temporaryDirectory,
		TaskDatabasePath:       filepath.Join(temporaryDirectory, "flow.sqlite"),
		AttendanceDatabasePath: filepath.Join(temporaryDirectory, "attendance.sqlite"),
		ClaimedAdminEmailPath:  filepath.Join(temporaryDirectory, "claimed-admin-email"),
	})
	if errorValue := service.writeWorkspaceSettingsFile(workspaceSettings{TimeZone: "Asia/Seoul", Language: workspaceLanguageEnglish}); errorValue != nil {
		t.Fatal(errorValue)
	}
	return service
}

func writeClaimedAdminEmailForTest(t *testing.T, service *Service, email string) {
	t.Helper()
	if errorValue := os.WriteFile(service.Configuration.ClaimedAdminEmailPath, []byte(email), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
}

