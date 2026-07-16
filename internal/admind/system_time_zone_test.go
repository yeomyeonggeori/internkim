package admind

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestResolveSystemTimeZoneUsesEnvironment(t *testing.T) {
	resolved := resolveSystemTimeZone(systemTimeZoneSource{
		environmentTimeZone: "Asia/Ho_Chi_Minh",
		localLocation:       time.FixedZone("Local", 0),
	})

	assertAuthoritativeSystemTimeZone(t, resolved, "Asia/Ho_Chi_Minh")
}

func TestResolveSystemTimeZoneUsesZoneInfoLink(t *testing.T) {
	rootDirectory := t.TempDir()
	zoneInfoPath := filepath.Join(rootDirectory, "usr", "share", "zoneinfo", "America", "New_York")
	if errorValue := os.MkdirAll(filepath.Dir(zoneInfoPath), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(zoneInfoPath, []byte("zoneinfo"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	localTimePath := filepath.Join(rootDirectory, "etc", "localtime")
	if errorValue := os.MkdirAll(filepath.Dir(localTimePath), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.Symlink(zoneInfoPath, localTimePath); errorValue != nil {
		t.Fatal(errorValue)
	}

	resolved := resolveSystemTimeZone(systemTimeZoneSource{
		localTimePath: localTimePath,
		localLocation: time.FixedZone("Local", 0),
	})

	assertAuthoritativeSystemTimeZone(t, resolved, "America/New_York")
}

func TestResolveSystemTimeZoneUsesTimeZoneFile(t *testing.T) {
	timeZonePath := filepath.Join(t.TempDir(), "timezone")
	if errorValue := os.WriteFile(timeZonePath, []byte("Asia/Ho_Chi_Minh\n"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}

	resolved := resolveSystemTimeZone(systemTimeZoneSource{
		timeZonePath:  timeZonePath,
		localLocation: time.FixedZone("Local", 0),
	})

	assertAuthoritativeSystemTimeZone(t, resolved, "Asia/Ho_Chi_Minh")
}

func TestResolveSystemTimeZoneUsesTimeZoneFileMatchingCopiedLocalTime(t *testing.T) {
	rootDirectory := t.TempDir()
	zoneInfoDirectory := filepath.Join(rootDirectory, "zoneinfo")
	zoneInfoPath := filepath.Join(zoneInfoDirectory, "Asia", "Ho_Chi_Minh")
	if errorValue := os.MkdirAll(filepath.Dir(zoneInfoPath), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	zoneInfoDocument := []byte("matching zoneinfo")
	if errorValue := os.WriteFile(zoneInfoPath, zoneInfoDocument, 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	localTimePath := filepath.Join(rootDirectory, "localtime")
	if errorValue := os.WriteFile(localTimePath, zoneInfoDocument, 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	timeZonePath := filepath.Join(rootDirectory, "timezone")
	if errorValue := os.WriteFile(timeZonePath, []byte("Asia/Ho_Chi_Minh\n"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}

	resolved := resolveSystemTimeZone(systemTimeZoneSource{
		localTimePath:     localTimePath,
		timeZonePath:      timeZonePath,
		zoneInfoDirectory: zoneInfoDirectory,
		localLocation:     time.FixedZone("Local", 0),
	})

	assertAuthoritativeSystemTimeZone(t, resolved, "Asia/Ho_Chi_Minh")
}

func TestResolveSystemTimeZoneRejectsTimeZoneFileMismatchingCopiedLocalTime(t *testing.T) {
	rootDirectory := t.TempDir()
	zoneInfoDirectory := filepath.Join(rootDirectory, "zoneinfo")
	zoneInfoPath := filepath.Join(zoneInfoDirectory, "Asia", "Ho_Chi_Minh")
	if errorValue := os.MkdirAll(filepath.Dir(zoneInfoPath), 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(zoneInfoPath, []byte("configured zoneinfo"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	localTimePath := filepath.Join(rootDirectory, "localtime")
	if errorValue := os.WriteFile(localTimePath, []byte("different zoneinfo"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	timeZonePath := filepath.Join(rootDirectory, "timezone")
	if errorValue := os.WriteFile(timeZonePath, []byte("Asia/Ho_Chi_Minh\n"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	localLocation := time.FixedZone("Local", 7*60*60)

	resolved := resolveSystemTimeZone(systemTimeZoneSource{
		localTimePath:     localTimePath,
		timeZonePath:      timeZonePath,
		zoneInfoDirectory: zoneInfoDirectory,
		localLocation:     localLocation,
	})

	assertNonAuthoritativeLocalTimeZone(t, resolved, localLocation)
}

func TestResolveSystemTimeZoneRejectsUnverifiableCopiedLocalTime(t *testing.T) {
	rootDirectory := t.TempDir()
	localTimePath := filepath.Join(rootDirectory, "localtime")
	if errorValue := os.WriteFile(localTimePath, []byte("copied zoneinfo"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	timeZonePath := filepath.Join(rootDirectory, "timezone")
	if errorValue := os.WriteFile(timeZonePath, []byte("Asia/Ho_Chi_Minh\n"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	localLocation := time.FixedZone("Local", 7*60*60)

	resolved := resolveSystemTimeZone(systemTimeZoneSource{
		localTimePath:     localTimePath,
		timeZonePath:      timeZonePath,
		zoneInfoDirectory: filepath.Join(rootDirectory, "missing-zoneinfo"),
		localLocation:     localLocation,
	})

	assertNonAuthoritativeLocalTimeZone(t, resolved, localLocation)
}

func TestResolveSystemTimeZoneUsesCanonicalLocalLocation(t *testing.T) {
	localLocation, errorValue := time.LoadLocation("America/Los_Angeles")
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	resolved := resolveSystemTimeZone(systemTimeZoneSource{localLocation: localLocation})

	assertAuthoritativeSystemTimeZone(t, resolved, "America/Los_Angeles")
}

func TestResolveSystemTimeZoneDoesNotGuessCopiedLocalTime(t *testing.T) {
	localTimePath := filepath.Join(t.TempDir(), "localtime")
	if errorValue := os.WriteFile(localTimePath, []byte("copied zoneinfo"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	localLocation := time.FixedZone("Local", 9*60*60)

	resolved := resolveSystemTimeZone(systemTimeZoneSource{
		localTimePath: localTimePath,
		localLocation: localLocation,
	})

	assertNonAuthoritativeLocalTimeZone(t, resolved, localLocation)
}

func TestResolveSystemTimeZoneDoesNotMapPOSIXEnvironment(t *testing.T) {
	localLocation := time.FixedZone("Local", -5*60*60)

	resolved := resolveSystemTimeZone(systemTimeZoneSource{
		environmentTimeZone: "EST5EDT",
		localLocation:       localLocation,
	})

	assertNonAuthoritativeLocalTimeZone(t, resolved, localLocation)
}

func assertAuthoritativeSystemTimeZone(t *testing.T, resolved resolvedTimeZone, expectedName string) {
	t.Helper()
	if resolved.name != expectedName {
		t.Fatalf("system time zone name = %q", resolved.name)
	}
	if resolved.location == nil {
		t.Fatal("system time zone location is nil")
	}
	if resolved.location.String() != resolved.name {
		t.Fatalf("system time zone location = %q, name = %q", resolved.location.String(), resolved.name)
	}
	if !resolved.isAuthoritative {
		t.Fatal("system time zone is not authoritative")
	}
}

func assertNonAuthoritativeLocalTimeZone(t *testing.T, resolved resolvedTimeZone, expectedLocation *time.Location) {
	t.Helper()
	if resolved.location != expectedLocation {
		t.Fatal("system time zone did not preserve the local location")
	}
	if resolved.name != "Local" {
		t.Fatalf("system time zone name = %q", resolved.name)
	}
	if resolved.isAuthoritative {
		t.Fatal("system time zone is authoritative")
	}
}
