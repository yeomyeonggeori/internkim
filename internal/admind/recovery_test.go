package admind

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSSHRecoveryRestartUsesSignedAllowlistedAction(t *testing.T) {
	service := newRecoveryTestService(t)
	commands := []string{}
	service.RunCommand = func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
		commands = append(commands, strings.TrimSpace(name+" "+strings.Join(arguments, " ")))
		switch strings.Join(append([]string{name}, arguments...), " ") {
		case "systemctl is-active ssh":
			return []byte("active\n"), nil
		case "systemctl is-active cloudflared-node-ssh":
			return []byte("inactive\n"), nil
		case "systemctl is-active cloudflared":
			return []byte("active\n"), nil
		case "systemctl restart cloudflared-node-ssh":
			return []byte("restarted\n"), nil
		case "journalctl -u ssh -u cloudflared-node-ssh -n 80 --no-pager":
			return []byte("Authorization: Bearer secret-token\nnode ok\n"), nil
		default:
			t.Fatalf("unexpected command %s %v", name, arguments)
			return nil, nil
		}
	}

	recorder := httptest.NewRecorder()
	request := signedRecoveryRequest(t, service, "restart-cloudflared-node-ssh", "nonce-1", time.Now().UTC())
	service.handleAdmin(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	if !containsString(commands, "systemctl restart cloudflared-node-ssh") {
		t.Fatalf("expected cloudflared-node-ssh restart, got %+v", commands)
	}
	if strings.Contains(recorder.Body.String(), "secret-token") || strings.Contains(recorder.Body.String(), "Bearer") {
		t.Fatalf("expected sensitive output redacted, got %s", recorder.Body.String())
	}
}

func TestSSHRecoveryRejectsUnsupportedAction(t *testing.T) {
	service := newRecoveryTestService(t)
	recorder := httptest.NewRecorder()
	request := signedRecoveryRequest(t, service, "restart-postgresql", "nonce-1", time.Now().UTC())

	service.handleAdmin(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestSSHRecoveryRejectsInvalidSignature(t *testing.T) {
	service := newRecoveryTestService(t)
	request := signedRecoveryRequest(t, service, "status", "nonce-1", time.Now().UTC())
	var payload sshRecoveryRequest
	if errorValue := json.NewDecoder(request.Body).Decode(&payload); errorValue != nil {
		t.Fatal(errorValue)
	}
	payload.Signature = "invalid"
	document, _ := json.Marshal(payload)
	request = httptest.NewRequest(http.MethodPost, "/admin/api/recovery/ssh-tunnel/restart", bytes.NewReader(document))
	recorder := httptest.NewRecorder()

	service.handleAdmin(recorder, request)

	if recorder.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
}

func TestSSHRecoveryRejectsStaleAndReplayedRequests(t *testing.T) {
	service := newRecoveryTestService(t)
	staleRecorder := httptest.NewRecorder()
	staleRequest := signedRecoveryRequest(t, service, "status", "nonce-stale", time.Now().UTC().Add(-10*time.Minute))
	service.handleAdmin(staleRecorder, staleRequest)
	if staleRecorder.Code != http.StatusForbidden {
		t.Fatalf("stale status = %d body = %s", staleRecorder.Code, staleRecorder.Body.String())
	}

	firstRecorder := httptest.NewRecorder()
	firstRequest := signedRecoveryRequest(t, service, "status", "nonce-replay", time.Now().UTC())
	service.handleAdmin(firstRecorder, firstRequest)
	if firstRecorder.Code != http.StatusOK {
		t.Fatalf("first status = %d body = %s", firstRecorder.Code, firstRecorder.Body.String())
	}

	secondRecorder := httptest.NewRecorder()
	secondRequest := signedRecoveryRequest(t, service, "status", "nonce-replay", time.Now().UTC())
	service.handleAdmin(secondRecorder, secondRequest)
	if secondRecorder.Code != http.StatusForbidden {
		t.Fatalf("replay status = %d body = %s", secondRecorder.Code, secondRecorder.Body.String())
	}
}

func newRecoveryTestService(t *testing.T) *Service {
	t.Helper()
	directoryPath := t.TempDir()
	fleetIDPath := filepath.Join(directoryPath, "fleet-id")
	fleetSecretPath := filepath.Join(directoryPath, "fleet-secret")
	writeFile(t, fleetIDPath, "fleet-1")
	writeFile(t, fleetSecretPath, "secret-1")
	service := NewService(Configuration{
		StateDirectory:  directoryPath,
		FleetIDPath:     fleetIDPath,
		FleetSecretPath: fleetSecretPath,
		AdminEmailPath:  writeTestFile(t, "admin@example.com"),
	})
	service.RunCommand = func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
		if name == "systemctl" && len(arguments) == 2 && arguments[0] == "is-active" {
			return []byte("active\n"), nil
		}
		if name == "journalctl" {
			return []byte("ok\n"), nil
		}
		return []byte("ok\n"), nil
	}
	return service
}

func signedRecoveryRequest(t *testing.T, service *Service, action string, nonce string, createdAt time.Time) *http.Request {
	t.Helper()
	timestamp := createdAt.Format(time.RFC3339)
	deviceID := strings.TrimSpace(readTrimmedFile(service.Configuration.FleetIDPath))
	secret := strings.TrimSpace(readTrimmedFile(service.Configuration.FleetSecretPath))
	payload := sshRecoveryRequest{
		fleetSignedRequest: fleetSignedRequest{
			Action:    action,
			DeviceID:  deviceID,
			Nonce:     nonce,
			Timestamp: timestamp,
			Signature: signFleetPayload(secret, action, deviceID, nonce, timestamp),
		},
	}
	document, errorValue := json.Marshal(payload)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return httptest.NewRequest(http.MethodPost, "/admin/api/recovery/ssh-tunnel/restart", bytes.NewReader(document))
}
