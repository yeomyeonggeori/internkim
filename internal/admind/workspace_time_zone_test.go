package admind

import (
	"errors"
	"testing"
	"time"
)

func TestAbsentWorkspaceSettingsDefaultToTheBusinessTimeZone(t *testing.T) {
	service := &Service{Configuration: Configuration{StateDirectory: t.TempDir()}}

	resolved := service.workspaceTimeZone()

	if resolved.name != workspaceBusinessTimeZone || !resolved.isAuthoritative {
		t.Fatalf("expected authoritative %s default, got %+v", workspaceBusinessTimeZone, resolved)
	}
}

func TestResolveWorkspaceTimeZoneUsesAuthoritativeSystemSetting(t *testing.T) {
	systemTimeZone := authoritativeTestTimeZone(t, "Asia/Seoul")

	resolved := resolveWorkspaceTimeZone(
		workspaceSettings{TimeZone: workspaceSystemTimeZone},
		nil,
		func() resolvedTimeZone { return systemTimeZone },
	)

	if resolved != systemTimeZone {
		t.Fatal("workspace system time zone did not preserve the resolved system setting")
	}
}

func TestResolveWorkspaceTimeZoneRejectsUnreadableExplicitSetting(t *testing.T) {
	systemTimeZone := authoritativeTestTimeZone(t, "Asia/Seoul")

	resolved := resolveWorkspaceTimeZone(
		workspaceSettings{},
		errors.New("workspace settings unavailable"),
		func() resolvedTimeZone { return systemTimeZone },
	)

	assertNonAuthoritativeWorkspaceTimeZone(t, resolved, systemTimeZone)
}

func TestResolveWorkspaceTimeZoneRejectsInvalidExplicitSetting(t *testing.T) {
	systemTimeZone := authoritativeTestTimeZone(t, "Asia/Seoul")

	resolved := resolveWorkspaceTimeZone(
		workspaceSettings{TimeZone: "Missing/Time_Zone"},
		nil,
		func() resolvedTimeZone { return systemTimeZone },
	)

	assertNonAuthoritativeWorkspaceTimeZone(t, resolved, systemTimeZone)
}

func authoritativeTestTimeZone(t *testing.T, timeZoneName string) resolvedTimeZone {
	t.Helper()
	location, errorValue := time.LoadLocation(timeZoneName)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return resolvedTimeZone{
		location:        location,
		name:            timeZoneName,
		isAuthoritative: true,
	}
}

func assertNonAuthoritativeWorkspaceTimeZone(
	t *testing.T,
	resolved resolvedTimeZone,
	expectedSystemTimeZone resolvedTimeZone,
) {
	t.Helper()
	if resolved.location != expectedSystemTimeZone.location {
		t.Fatal("workspace time zone did not preserve the system location")
	}
	if resolved.name != expectedSystemTimeZone.name {
		t.Fatalf("workspace time zone name = %q", resolved.name)
	}
	if resolved.isAuthoritative {
		t.Fatal("workspace time zone is authoritative")
	}
}
