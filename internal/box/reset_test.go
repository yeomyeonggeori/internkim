package box

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type fakeReleasePlane struct {
	refusalsLeft int
	releases     int
}

func (plane *fakeReleasePlane) serve(t *testing.T) *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/box/release" {
			t.Errorf("unexpected request to %s", request.URL.Path)
			return
		}
		if plane.refusalsLeft > 0 {
			plane.refusalsLeft--
			http.Error(writer, "the central plane is not configured", http.StatusInternalServerError)
			return
		}
		plane.releases++
		writer.Write([]byte(`{"wasClaimed":true}`))
	}))
	t.Cleanup(server.Close)
	return server
}

func resetDaemon(t *testing.T, plane *fakeReleasePlane, isAsked bool) (Daemon, *int) {
	t.Helper()
	root := t.TempDir()
	places := Places{StateDirectoryPath: filepath.Join(root, "box"), ResetRequestPath: filepath.Join(root, "boot", "internkim-reset")}
	if errorValue := os.MkdirAll(places.StateDirectoryPath, 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(filepath.Join(places.StateDirectoryPath, companyMarkerFileName), []byte(sampleCompanyID+"\n"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	if isAsked {
		if errorValue := os.MkdirAll(filepath.Dir(places.ResetRequestPath), 0o755); errorValue != nil {
			t.Fatal(errorValue)
		}
		if errorValue := os.WriteFile(places.ResetRequestPath, nil, 0o644); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	locks := 0
	return Daemon{
		Client:            Client{AppURL: plane.serve(t).URL},
		Places:            places,
		LockAdminPassword: func(context.Context) error { locks++; return nil },
		Sleep:             func(context.Context, time.Duration) error { return nil },
	}, &locks
}

func TestAResetFileReleasesTheBoxLocksAdminAndIsRemoved(t *testing.T) {
	plane := &fakeReleasePlane{}
	daemon, locks := resetDaemon(t, plane, true)

	if errorValue := daemon.resetIfAsked(context.Background(), freshTestIdentity(t)); errorValue != nil {
		t.Fatal(errorValue)
	}

	if plane.releases != 1 || *locks != 1 {
		t.Fatalf("releases = %d, admin locks = %d", plane.releases, *locks)
	}
	if daemon.installedCompany() != "" {
		t.Fatalf("a reset box still names %q as its company", daemon.installedCompany())
	}
	if _, errorValue := os.Stat(daemon.Places.ResetRequestPath); !errors.Is(errorValue, os.ErrNotExist) {
		t.Fatalf("the reset file is still there: %v", errorValue)
	}
}

func TestAResetKeepsAskingUntilThePlaneReleasesTheBox(t *testing.T) {
	plane := &fakeReleasePlane{refusalsLeft: 2}
	daemon, locks := resetDaemon(t, plane, true)
	sleeps := 0
	daemon.Sleep = func(context.Context, time.Duration) error {
		sleeps++
		if _, errorValue := os.Stat(daemon.Places.ResetRequestPath); errorValue != nil {
			t.Fatal("the reset file was removed before the plane released the box")
		}
		return nil
	}

	if errorValue := daemon.resetIfAsked(context.Background(), freshTestIdentity(t)); errorValue != nil {
		t.Fatal(errorValue)
	}

	if sleeps != 2 || plane.releases != 1 || *locks != 1 {
		t.Fatalf("sleeps = %d, releases = %d, admin locks = %d", sleeps, plane.releases, *locks)
	}
}

func TestWithoutAResetFileTheBoxKeepsItsCompany(t *testing.T) {
	plane := &fakeReleasePlane{}
	daemon, locks := resetDaemon(t, plane, false)

	if errorValue := daemon.resetIfAsked(context.Background(), freshTestIdentity(t)); errorValue != nil {
		t.Fatal(errorValue)
	}

	if plane.releases != 0 || *locks != 0 || daemon.installedCompany() != sampleCompanyID {
		t.Fatalf("releases = %d, admin locks = %d, company = %q", plane.releases, *locks, daemon.installedCompany())
	}
}
