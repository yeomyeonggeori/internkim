package admind

import (
	"os"
	"path/filepath"
	"testing"
)

func newCalendarTestService(t *testing.T) *Service {
	t.Helper()
	rootPath := t.TempDir()
	adminUIPath := filepath.Join(rootPath, "admin-ui")
	if errorValue := os.MkdirAll(adminUIPath, 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(filepath.Join(adminUIPath, "index.html"), []byte("<script></script>"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	service := NewService(Configuration{
		StateDirectory:       filepath.Join(rootPath, "state", "admin"),
		CalendarDatabasePath: filepath.Join(rootPath, "state", "calendar.sqlite"),
		TaskDatabasePath:     filepath.Join(rootPath, "state", "flow.sqlite"),
		AdminEmailPath:       writeTestFile(t, "admin@example.com"),
	})
	return service
}
