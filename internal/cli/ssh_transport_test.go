package cli

import (
	"errors"
	"strings"
	"testing"
)

var errRetryableForTest = errors.New("ssh refused this attempt")

func TestEverySSHTransportPinsPasswordAuthentication(t *testing.T) {
	client := &sshClient{user: "admin", host: "127.0.0.1", port: "22", pass: "secret", sshpassBin: "sshpass"}
	transports := map[string]string{
		"ssh":   strings.Join(client.sshArgs(), " "),
		"scp":   strings.Join(client.scpArgs(), " "),
		"rsync": client.rsyncSSHCommand("ssh"),
	}
	for name, arguments := range transports {
		if !strings.Contains(arguments, "PreferredAuthentications=password") ||
			!strings.Contains(arguments, "PubkeyAuthentication=no") {
			t.Errorf("the %s transport offers keys before the password it was given: %s", name, arguments)
		}
	}
}

func TestASparseUploadRetriesTheFailureSSHCallsTransient(t *testing.T) {
	if !isRetryableSSHFailure("Permission denied, please try again.") {
		t.Fatal("this test is asserting against the wrong phrase")
	}
	attempts := 0
	output, errorValue := retryWhileSSHFailureIsTransient(func() (string, error) {
		attempts++
		if attempts < 3 {
			return "Permission denied, please try again.", errRetryableForTest
		}
		return "sent", nil
	})
	if errorValue != nil || output != "sent" {
		t.Fatalf("a transient refusal ended the upload: %q %v", output, errorValue)
	}
	if attempts != 3 {
		t.Fatalf("expected three attempts, got %d", attempts)
	}
}
