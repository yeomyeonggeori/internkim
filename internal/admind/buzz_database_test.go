package admind

import (
	"database/sql"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/lib/pq"
	blueclawruntime "github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

func TestTheBuzzDatabaseIsOpenedOnceAndBounded(t *testing.T) {
	service := NewService(Configuration{BuzzDatabaseURL: "postgres://nobody@127.0.0.1:1/buzz"})

	first, errorValue := service.buzzDatabase()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	second, errorValue := service.buzzDatabase()
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if first != second {
		t.Fatal("two calls got two handles, so the buzz database has no owner")
	}
	if openConnections := first.Stats().MaxOpenConnections; openConnections != blueclawruntime.AdminDatabaseConnections {
		t.Fatalf("the pool may open %d connections, wanted the %d the budget grants admind", openConnections, blueclawruntime.AdminDatabaseConnections)
	}
	assertBuzzPoolSetting(t, first, "maxIdleCount", int64(blueclawruntime.AdminDatabaseConnections))
	assertBuzzPoolSetting(t, first, "maxIdleTime", int64(blueclawruntime.DatabaseConnectionMaxIdleTime))
	assertBuzzPoolSetting(t, first, "maxLifetime", int64(blueclawruntime.DatabaseConnectionMaxLifetime))
}

func TestTheCompanyHostReachesTheMessengerDatabaseOnItsSocket(t *testing.T) {
	layout := blueclawruntime.LinuxCompanyHostLayout()
	configuration, errorValue := postgresConfiguration(layout.DatabaseURL("internkim", "a+/=password", blueclawruntime.BuzzRelayDatabaseName))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if configuration.Host != layout.DatabaseSocketDirectory {
		t.Fatalf("admind dials %q, and the host's database listens only on %s", configuration.Host, layout.DatabaseSocketDirectory)
	}
	if configuration.SSLMode != pq.SSLModeDisable {
		t.Fatalf("admind asks the socket for SSL (%q), which the host's database does not offer", configuration.SSLMode)
	}
	if configuration.Password != "a+/=password" {
		t.Fatalf("the password arrived as %q", configuration.Password)
	}
}

func TestATCPDatabaseURLKeepsItsHost(t *testing.T) {
	configuration, errorValue := postgresConfiguration("postgres://nobody@127.0.0.1:5433/buzz?sslmode=disable")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if configuration.Host != "127.0.0.1" || configuration.Port != 5433 {
		t.Fatalf("a TCP URL became %s:%d", configuration.Host, configuration.Port)
	}
}

func TestNothingElseOpensTheBuzzDatabase(t *testing.T) {
	entries, errorValue := os.ReadDir(".")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	openers := []string{}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") || entry.Name() == "buzz_database.go" {
			continue
		}
		document, errorValue := os.ReadFile(filepath.Join(".", entry.Name()))
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if strings.Contains(string(document), `sql.Open("postgres"`) {
			openers = append(openers, entry.Name())
		}
	}
	if len(openers) > 0 {
		t.Fatalf("these files open the buzz database themselves instead of asking its owner: %s", strings.Join(openers, ", "))
	}
}

func assertBuzzPoolSetting(t *testing.T, database *sql.DB, fieldName string, want int64) {
	t.Helper()
	field := reflect.ValueOf(database).Elem().FieldByName(fieldName)
	if !field.IsValid() {
		t.Fatalf("database/sql no longer keeps its pool settings in a field called %s", fieldName)
	}
	if field.Int() != want {
		t.Fatalf("%s is %s, wanted %s", fieldName, time.Duration(field.Int()), time.Duration(want))
	}
}
