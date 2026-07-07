package admind

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"
)

func listenOnEphemeralPort(t *testing.T) (net.Listener, int) {
	listener, errorValue := net.Listen("tcp", "127.0.0.1:0")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return listener, listener.Addr().(*net.TCPAddr).Port
}

func TestEnsureSitePocketBaseRunningStartsUnitOnce(t *testing.T) {
	listener, port := listenOnEphemeralPort(t)
	defer listener.Close()
	service := &Service{}
	commands := []string{}
	service.RunCommand = func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
		commands = append(commands, strings.Join(append([]string{name}, arguments...), " "))
		return nil, nil
	}
	site := &SiteRecord{SiteID: "site-1", Port: port}

	if errorValue := service.ensureSitePocketBaseRunning(context.Background(), site); errorValue != nil {
		t.Fatalf("first ensure failed: %v", errorValue)
	}
	if errorValue := service.ensureSitePocketBaseRunning(context.Background(), site); errorValue != nil {
		t.Fatalf("second ensure failed: %v", errorValue)
	}

	startCount := 0
	for _, command := range commands {
		if strings.Contains(command, "systemctl start") {
			startCount++
		}
	}
	if startCount != 1 {
		t.Fatalf("expected one systemctl start, got %d: %v", startCount, commands)
	}
}

func TestIdleJanitorStopsOnlyIdleRuntimes(t *testing.T) {
	service := &Service{}
	commands := []string{}
	service.RunCommand = func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
		commands = append(commands, strings.Join(append([]string{name}, arguments...), " "))
		return nil, nil
	}

	idleActivity := service.siteRuntimeActivityFor("idle-site")
	idleActivity.isRunning = true
	idleActivity.lastRequestAt = time.Now().Add(-sitePocketBaseIdleTimeout - time.Minute)

	busyActivity := service.siteRuntimeActivityFor("busy-site")
	busyActivity.isRunning = true
	busyActivity.inflightCount = 1
	busyActivity.lastRequestAt = time.Now().Add(-sitePocketBaseIdleTimeout - time.Minute)

	freshActivity := service.siteRuntimeActivityFor("fresh-site")
	freshActivity.isRunning = true
	freshActivity.lastRequestAt = time.Now()

	service.stopIdleSitePocketBaseRuntimes(context.Background())

	joined := strings.Join(commands, "\n")
	if !strings.Contains(joined, "idle-site") {
		t.Fatalf("idle site was not stopped: %v", commands)
	}
	if strings.Contains(joined, "busy-site") || strings.Contains(joined, "fresh-site") {
		t.Fatalf("busy or fresh site was stopped: %v", commands)
	}
	if idleActivity.isRunning {
		t.Fatal("idle site still marked running")
	}
}

func TestFinishSitePocketBaseRequestDecrementsInflight(t *testing.T) {
	service := &Service{}
	activity := service.siteRuntimeActivityFor("site-1")
	activity.inflightCount = 2

	service.finishSitePocketBaseRequest("site-1")

	if activity.inflightCount != 1 {
		t.Fatalf("expected inflight 1, got %d", activity.inflightCount)
	}
}
