package boxwifi

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sampleCompanySlug = "sample-company"

func TestAClaimedBoxIsNamedAfterItsCompanySlug(t *testing.T) {
	if got := HostNameFor(sampleCompanySlug); got != sampleCompanySlug {
		t.Fatalf("host name = %q, want %q", got, sampleCompanySlug)
	}
}

func TestABoxWithoutAUsableSlugIsNamedKimmini(t *testing.T) {
	for _, slug := range []string{"", "Sample", "-sample", "sample-", "sample_company", "sample.company", strings.Repeat("a", 64)} {
		if got := HostNameFor(slug); got != SetupNetworkName {
			t.Errorf("host name for %q = %q, want %q", slug, got, SetupNetworkName)
		}
	}
}

func TestNamerRenamesOnlyAComputerNamedSomethingElse(t *testing.T) {
	for _, testCase := range []struct {
		current     string
		wantsRename bool
	}{
		{current: "kimmini", wantsRename: true},
		{current: sampleCompanySlug, wantsRename: false},
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
		if errorValue := namer.Name(context.Background(), sampleCompanySlug); errorValue != nil {
			t.Fatal(errorValue)
		}
		renamed := containsCall(calls, []string{"hostnamectl", "set-hostname", sampleCompanySlug})
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
		if string(hosts) != "127.0.0.1\tlocalhost\n127.0.1.1\t"+sampleCompanySlug+"\n" {
			t.Errorf("hosts after naming a computer named %q = %q, want 127.0.1.1 to follow the new name", testCase.current, hosts)
		}
	}
}

func TestHostsGainsALoopbackLineWhenItHadNone(t *testing.T) {
	if got := hostsWithLoopbackName("127.0.0.1\tlocalhost\n", sampleCompanySlug); got != "127.0.0.1\tlocalhost\n127.0.1.1\t"+sampleCompanySlug+"\n" {
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

	if errorValue := namer.Name(context.Background(), sampleCompanySlug); errorValue == nil {
		t.Fatal("expected the failed restart to be reported")
	}

	hosts, errorValue := os.ReadFile(hostsPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if string(hosts) != "127.0.1.1\t"+sampleCompanySlug+"\n" {
		t.Fatalf("hosts = %q, want 127.0.1.1 to follow the new name before the restart is tried", hosts)
	}
}
