package box

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type releasePlane struct {
	role        Role
	shortRounds int
	aimed       int
}

func (plane *releasePlane) serve(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/box/session":
			json.NewEncoder(writer).Encode(map[string]any{"role": plane.role})
		case "/api/box/release":
			plane.aimed++
			if plane.role == RoleStandby {
				http.Error(writer, "only the box that serves the company aims its standby boxes at a release", http.StatusConflict)
				return
			}
			shortOfIt := []BoxShortOfRelease{}
			if plane.aimed <= plane.shortRounds {
				shortOfIt = append(shortOfIt, BoxShortOfRelease{HostName: "standby-sample", PackageVersion: "v0.0.1"})
			}
			json.NewEncoder(writer).Encode(map[string]any{"standbyBoxesShortOfIt": shortOfIt})
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func releaseOrderFor(t *testing.T, server *httptest.Server, now func() time.Time, sleep func(time.Duration)) ReleaseOrder {
	t.Helper()
	return ReleaseOrder{
		Client:         Client{AppURL: server.URL, HTTPClient: server.Client(), Now: now},
		Identity:       identityWithEncryptionSeed(t, strings.Repeat("A", 43)),
		PackageVersion: "v0.0.1",
		Wait:           time.Minute,
		Interval:       15 * time.Second,
		Sleep:          sleep,
	}
}

func TestAStandbyBoxTakesTheReleaseItWasAimedAtWithoutAimingOthers(t *testing.T) {
	plane := &releasePlane{role: RoleStandby}
	order := releaseOrderFor(t, plane.serve(t), time.Now, func(time.Duration) { t.Fatal("a standby box has no one to wait for") })

	if errorValue := order.StandbyBoxesTakeItFirst(context.Background(), "v0.0.2"); errorValue != nil {
		t.Fatalf("a standby box must go on to install the release it follows, got %v", errorValue)
	}
	if plane.aimed != 0 {
		t.Fatalf("a standby box must not aim the company's standby boxes, but it asked %d times", plane.aimed)
	}
}

func TestTheServiceBoxWaitsForItsStandbyBoxesBeforeItInstalls(t *testing.T) {
	plane := &releasePlane{role: RoleService, shortRounds: 2}
	slept := 0
	order := releaseOrderFor(t, plane.serve(t), time.Now, func(time.Duration) { slept++ })

	if errorValue := order.StandbyBoxesTakeItFirst(context.Background(), "v0.0.2"); errorValue != nil {
		t.Fatalf("the service box must install once its standby boxes run the release, got %v", errorValue)
	}
	if plane.aimed != 3 || slept != 2 {
		t.Fatalf("expected two rounds of waiting before the standby caught up, got %d aims and %d sleeps", plane.aimed, slept)
	}
}

func TestTheServiceBoxStaysWhereItIsWhenAStandbyNeverCatchesUp(t *testing.T) {
	plane := &releasePlane{role: RoleService, shortRounds: 1000}
	clock := time.Unix(1_800_000_000, 0)
	order := releaseOrderFor(t, plane.serve(t), func() time.Time { return clock }, func(interval time.Duration) { clock = clock.Add(interval) })

	errorValue := order.StandbyBoxesTakeItFirst(context.Background(), "v0.0.2")

	if errorValue == nil || !strings.Contains(errorValue.Error(), "standby-sample") {
		t.Fatalf("the service box must refuse to install past a standby that never took the release, got %v", errorValue)
	}
}
