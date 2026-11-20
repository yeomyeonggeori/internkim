package box

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

type fakeDisconnectingPlane struct {
	refusalsLeft     int
	disconnectedWith []string
}

func (plane *fakeDisconnectingPlane) serve(t *testing.T) *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/box/disconnect" {
			t.Errorf("unexpected request to %s", request.URL.Path)
			return
		}
		if plane.refusalsLeft > 0 {
			plane.refusalsLeft--
			http.Error(writer, "the central plane is not configured", http.StatusInternalServerError)
			return
		}
		var body disconnection
		json.NewDecoder(request.Body).Decode(&body)
		plane.disconnectedWith = append(plane.disconnectedWith, body.ConnectionKey)
		writer.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(server.Close)
	return server
}

func resetDaemon(t *testing.T, plane *fakeDisconnectingPlane) (Daemon, *int) {
	t.Helper()
	root := t.TempDir()
	places := Places{
		StateDirectoryPath:   filepath.Join(root, "box"),
		ConnectionFilePath:   filepath.Join(root, "current", "connection.json"),
		BootResetRequestPath: filepath.Join(root, "boot", "internkim-reset"),
	}
	if errorValue := os.MkdirAll(places.StateDirectoryPath, 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := MarkInstalledCompany(places.StateDirectoryPath, sampleCompanyID); errorValue != nil {
		t.Fatal(errorValue)
	}
	locks := 0
	return Daemon{
		Client:            Client{AppURL: plane.serve(t).URL},
		Places:            places,
		LockAdminPassword: func(context.Context) error { locks++; return nil },
		Sleep:             func(context.Context, time.Duration) error { return nil },
	}, &locks
}

func askThroughTheBootPartition(t *testing.T, daemon Daemon) {
	t.Helper()
	writeConnectionFile(t, daemon.Places.BootResetRequestPath, nil)
}

func askThroughTheCommand(t *testing.T, daemon Daemon) {
	t.Helper()
	if errorValue := AskForReset(daemon.Places.StateDirectoryPath); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func requireNoResetRequest(t *testing.T, daemon Daemon) {
	t.Helper()
	for _, path := range daemon.resetRequestPaths() {
		if _, errorValue := os.Stat(path); !errors.Is(errorValue, os.ErrNotExist) {
			t.Fatalf("the reset request %s is still there: %v", path, errorValue)
		}
	}
}

func TestEveryWayOfAskingResetsTheBoxTheSameWay(t *testing.T) {
	for name, ask := range map[string]func(*testing.T, Daemon){
		"boot partition": askThroughTheBootPartition,
		"command":        askThroughTheCommand,
	} {
		t.Run(name, func(t *testing.T) {
			plane := &fakeDisconnectingPlane{}
			daemon, locks := resetDaemon(t, plane)
			ask(t, daemon)

			if errorValue := daemon.resetIfAsked(context.Background(), freshTestIdentity(t)); errorValue != nil {
				t.Fatal(errorValue)
			}

			if !slices.Equal(plane.disconnectedWith, []string{""}) || *locks != 1 {
				t.Fatalf("disconnected with %q, admin locks = %d, want one disconnection carrying no connection key and one lock", plane.disconnectedWith, *locks)
			}
			if daemon.installedCompany() != "" {
				t.Fatalf("a reset box still names %q as its company", daemon.installedCompany())
			}
			requireNoResetRequest(t, daemon)
		})
	}
}

func TestAResetKeepsAskingUntilThePlaneDisconnectsTheBox(t *testing.T) {
	plane := &fakeDisconnectingPlane{refusalsLeft: 2}
	daemon, locks := resetDaemon(t, plane)
	askThroughTheCommand(t, daemon)
	sleeps := 0
	daemon.Sleep = func(context.Context, time.Duration) error {
		sleeps++
		if !daemon.isResetAsked() {
			t.Fatal("the reset request was removed before the plane disconnected the box")
		}
		return nil
	}

	if errorValue := daemon.resetIfAsked(context.Background(), freshTestIdentity(t)); errorValue != nil {
		t.Fatal(errorValue)
	}

	if sleeps != 2 || len(plane.disconnectedWith) != 1 || *locks != 1 {
		t.Fatalf("sleeps = %d, disconnections = %d, admin locks = %d", sleeps, len(plane.disconnectedWith), *locks)
	}
}

func TestWithoutAResetRequestTheBoxKeepsItsCompany(t *testing.T) {
	plane := &fakeDisconnectingPlane{}
	daemon, locks := resetDaemon(t, plane)

	if errorValue := daemon.resetIfAsked(context.Background(), freshTestIdentity(t)); errorValue != nil {
		t.Fatal(errorValue)
	}

	if len(plane.disconnectedWith) != 0 || *locks != 0 || daemon.installedCompany() != sampleCompanyID {
		t.Fatalf("disconnections = %d, admin locks = %d, company = %q", len(plane.disconnectedWith), *locks, daemon.installedCompany())
	}
}

func TestAWithdrawnResetRequestResetsNothing(t *testing.T) {
	plane := &fakeDisconnectingPlane{}
	daemon, _ := resetDaemon(t, plane)
	askThroughTheCommand(t, daemon)

	if errorValue := WithdrawResetRequest(daemon.Places.StateDirectoryPath); errorValue != nil {
		t.Fatal(errorValue)
	}

	if daemon.isResetAsked() {
		t.Fatal("a withdrawn reset request still asks for a reset")
	}
}
