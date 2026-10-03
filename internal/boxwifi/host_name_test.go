package boxwifi

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const sampleBoxPublicKey = "DfNjfpwsVUGbJ9TutwdZmKHzIFi7_JfUwHtI4uxrYuA"

func TestHostNameEndsInTheSameFourCharactersAsTheSetupNetwork(t *testing.T) {
	setupNetworkName := SetupNetworkNameFor(sampleBoxPublicKey, time.Time{})
	if got := HostNameFor(sampleBoxPublicKey); got != strings.ToLower(setupNetworkName) {
		t.Fatalf("host name = %q, want %q lowercased", got, setupNetworkName)
	}
	if got := HostNameFor(sampleBoxPublicKey); got != "kimmini-ryua" {
		t.Fatalf("host name = %q, want kimmini-ryua", got)
	}
}

func TestHostNameTurnsKeyCharactersAHostNameCannotHoldIntoHyphens(t *testing.T) {
	cases := map[string]string{
		"keyEndingFh_s": "kimmini-fh-s",
		"keyEndingFh-s": "kimmini-fh-s",
		"keyEndingFhs_": "kimmini-fhs",
		"keyEnding_-_-": "kimmini",
		"ab":            "kimmini",
	}
	for key, want := range cases {
		if got := HostNameFor(key); got != want {
			t.Errorf("host name for %q = %q, want %q", key, got, want)
		}
	}
}

func TestNamerRenamesOnlyAComputerNamedSomethingElse(t *testing.T) {
	for _, testCase := range []struct {
		current     string
		wantsRename bool
	}{
		{current: "kimmini", wantsRename: true},
		{current: "kimmini-ryua", wantsRename: false},
	} {
		var calls [][]string
		hostsPath := filepath.Join(t.TempDir(), "hosts")
		if errorValue := os.WriteFile(hostsPath, []byte("127.0.0.1\tlocalhost\n127.0.1.1\tkimmini\n"), 0o644); errorValue != nil {
			t.Fatal(errorValue)
		}
		namer := HostNamer{
			Run: func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
				calls = append(calls, append([]string{name}, arguments...))
				return nil, nil
			},
			CurrentHostName: func() (string, error) { return testCase.current, nil },
			HostsPath:       hostsPath,
		}
		if errorValue := namer.Name(context.Background(), sampleBoxPublicKey); errorValue != nil {
			t.Fatal(errorValue)
		}
		renamed := containsCall(calls, []string{"hostnamectl", "set-hostname", "kimmini-ryua"})
		if renamed != testCase.wantsRename {
			t.Errorf("a computer named %q was renamed = %t, want %t; calls = %v", testCase.current, renamed, testCase.wantsRename, calls)
		}
		if announcesAgain := containsCall(calls, []string{"systemctl", "try-restart", multicastDNSService}); announcesAgain != testCase.wantsRename {
			t.Errorf("a computer named %q restarted %s = %t, want it only after a rename", testCase.current, multicastDNSService, announcesAgain)
		}
		hosts, errorValue := os.ReadFile(hostsPath)
		if errorValue != nil {
			t.Fatal(errorValue)
		}
		if string(hosts) != "127.0.0.1\tlocalhost\n127.0.1.1\tkimmini-ryua\n" {
			t.Errorf("hosts after naming a computer named %q = %q, want 127.0.1.1 to follow the new name", testCase.current, hosts)
		}
	}
}

func TestHostsGainsALoopbackLineWhenItHadNone(t *testing.T) {
	if got := hostsWithLoopbackName("127.0.0.1\tlocalhost\n", "kimmini-ryua"); got != "127.0.0.1\tlocalhost\n127.0.1.1\tkimmini-ryua\n" {
		t.Fatalf("hosts = %q", got)
	}
}

func TestHostsFollowsTheNewNameEvenWhenMulticastDNSWillNotRestart(t *testing.T) {
	hostsPath := filepath.Join(t.TempDir(), "hosts")
	if errorValue := os.WriteFile(hostsPath, []byte("127.0.1.1\tkimmini\n"), 0o644); errorValue != nil {
		t.Fatal(errorValue)
	}
	namer := HostNamer{
		Run: func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
			if name == "systemctl" {
				return nil, errors.New("avahi-daemon.service failed")
			}
			return nil, nil
		},
		CurrentHostName: func() (string, error) { return "kimmini", nil },
		HostsPath:       hostsPath,
	}

	if errorValue := namer.Name(context.Background(), sampleBoxPublicKey); errorValue == nil {
		t.Fatal("expected the failed restart to be reported")
	}

	hosts, errorValue := os.ReadFile(hostsPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if string(hosts) != "127.0.1.1\tkimmini-ryua\n" {
		t.Fatalf("hosts = %q, want 127.0.1.1 to follow the new name before the restart is tried", hosts)
	}
}
