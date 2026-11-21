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
		if got := HostNameFor(slug); got != EmptyBoxHostName {
			t.Errorf("host name for %q = %q, want %q", slug, got, EmptyBoxHostName)
		}
	}
}

type namerCalls struct {
	calls [][]string
}

func (recorded *namerCalls) run(ctx context.Context, name string, arguments ...string) ([]byte, error) {
	recorded.calls = append(recorded.calls, append([]string{name}, arguments...))
	return nil, nil
}

func namerFor(t *testing.T, current string, given string, recorded *namerCalls) HostNamer {
	t.Helper()
	stateDirectoryPath := t.TempDir()
	if given != "" {
		if errorValue := os.WriteFile(filepath.Join(stateDirectoryPath, givenHostNameFile), []byte(given+"\n"), 0o600); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	hostsPath := filepath.Join(t.TempDir(), "hosts")
	if errorValue := os.WriteFile(hostsPath, []byte("127.0.0.1\tlocalhost\n127.0.1.1\t"+current+"\n"), 0o644); errorValue != nil {
		t.Fatal(errorValue)
	}
	return HostNamer{
		Run:                recorded.run,
		CurrentHostName:    func() (string, error) { return current, nil },
		HostsPath:          hostsPath,
		StateDirectoryPath: stateDirectoryPath,
	}
}

func TestOnlyAHostNameInternKimGaveIsRenamed(t *testing.T) {
	for _, testCase := range []struct {
		name        string
		current     string
		given       string
		wantsRename bool
	}{
		{name: "an empty Kim mini", current: "kimmini", wantsRename: true},
		{name: "a computer InternKim named for an earlier company", current: "earlier-company", given: "earlier-company", wantsRename: true},
		{name: "a computer someone named", current: "jetson-host", wantsRename: false},
		{name: "a computer someone renamed after InternKim named it", current: "front-desk", given: "earlier-company", wantsRename: false},
		{name: "a numbered kimmini nobody recorded", current: "kimmini-2", wantsRename: false},
	} {
		var recorded namerCalls
		namer := namerFor(t, testCase.current, testCase.given, &recorded)
		if errorValue := namer.Name(context.Background(), sampleCompanySlug); errorValue != nil {
			t.Fatal(errorValue)
		}
		renamed := containsCall(recorded.calls, []string{"hostnamectl", "set-hostname", sampleCompanySlug})
		if renamed != testCase.wantsRename {
			t.Errorf("%s (%q) was renamed = %t, want %t; calls = %v", testCase.name, testCase.current, renamed, testCase.wantsRename, recorded.calls)
		}
		if announcesAgain := containsCall(recorded.calls, []string{"systemctl", "try-restart", multicastDNSService}); announcesAgain != testCase.wantsRename {
			t.Errorf("%s restarted %s = %t, want it only after a rename", testCase.name, multicastDNSService, announcesAgain)
		}
	}
}

func TestAComputerSomeoneNamedIsLeftExactlyAsItWas(t *testing.T) {
	var recorded namerCalls
	namer := namerFor(t, "jetson-host", "", &recorded)
	if errorValue := namer.Name(context.Background(), sampleCompanySlug); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(recorded.calls) != 0 {
		t.Fatalf("naming a computer someone named ran %v", recorded.calls)
	}
	hosts, errorValue := os.ReadFile(namer.HostsPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if string(hosts) != "127.0.0.1\tlocalhost\n127.0.1.1\tjetson-host\n" {
		t.Fatalf("hosts = %q, want it untouched", hosts)
	}
	if _, errorValue := os.Stat(namer.givenNamePath()); !errors.Is(errorValue, os.ErrNotExist) {
		t.Fatalf("a computer someone named was recorded as named by InternKim: %v", errorValue)
	}
}

func TestANameInternKimGivesIsRecordedSoItCanBeTakenBack(t *testing.T) {
	var recorded namerCalls
	namer := namerFor(t, "kimmini", "", &recorded)
	if errorValue := namer.Name(context.Background(), sampleCompanySlug); errorValue != nil {
		t.Fatal(errorValue)
	}
	if given := namer.givenName(); given != sampleCompanySlug {
		t.Fatalf("the recorded host name = %q, want %q", given, sampleCompanySlug)
	}

	renamed := namerFor(t, sampleCompanySlug, sampleCompanySlug, &recorded)
	if errorValue := renamed.Name(context.Background(), ""); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !containsCall(recorded.calls, []string{"hostnamectl", "set-hostname", EmptyBoxHostName}) {
		t.Fatalf("a disconnected computer InternKim named was not named %s again; calls = %v", EmptyBoxHostName, recorded.calls)
	}
	if given := renamed.givenName(); given != EmptyBoxHostName {
		t.Fatalf("the recorded host name after disconnect = %q, want %q", given, EmptyBoxHostName)
	}
}

func TestAComputerAlreadyNamedForItsCompanyOnlyPointsTheLoopbackAtIt(t *testing.T) {
	var recorded namerCalls
	namer := namerFor(t, sampleCompanySlug, sampleCompanySlug, &recorded)
	if errorValue := os.WriteFile(namer.HostsPath, []byte("127.0.0.1\tlocalhost\n127.0.1.1\tkimmini\n"), 0o644); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := namer.Name(context.Background(), sampleCompanySlug); errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(recorded.calls) != 0 {
		t.Fatalf("a computer already named for its company ran %v", recorded.calls)
	}
	hosts, errorValue := os.ReadFile(namer.HostsPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if string(hosts) != "127.0.0.1\tlocalhost\n127.0.1.1\t"+sampleCompanySlug+"\n" {
		t.Fatalf("hosts = %q, want 127.0.1.1 to follow the name", hosts)
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
		CurrentHostName:    func() (string, error) { return "kimmini", nil },
		HostsPath:          hostsPath,
		StateDirectoryPath: t.TempDir(),
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
