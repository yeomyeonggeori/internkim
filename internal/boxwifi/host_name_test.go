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

type namedComputer struct {
	hostName  string
	hostsPath string
	calls     [][]string
	namer     HostNamer
}

func aComputerNamed(t *testing.T, hostName string) *namedComputer {
	t.Helper()
	computer := &namedComputer{hostName: hostName, hostsPath: filepath.Join(t.TempDir(), "hosts")}
	if errorValue := os.WriteFile(computer.hostsPath, []byte("127.0.0.1\tlocalhost\n127.0.1.1\t"+hostName+"\n"), 0o644); errorValue != nil {
		t.Fatal(errorValue)
	}
	computer.namer = HostNamer{
		Run: func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
			computer.calls = append(computer.calls, append([]string{name}, arguments...))
			if name == "hostnamectl" {
				computer.hostName = arguments[len(arguments)-1]
			}
			return nil, nil
		},
		CurrentHostName:    func() (string, error) { return computer.hostName, nil },
		HostsPath:          computer.hostsPath,
		StateDirectoryPath: t.TempDir(),
	}
	return computer
}

func (computer *namedComputer) nameFor(t *testing.T, companySlug string) {
	t.Helper()
	if errorValue := computer.namer.Name(context.Background(), companySlug); errorValue != nil {
		t.Fatal(errorValue)
	}
}

func (computer *namedComputer) hosts(t *testing.T) string {
	t.Helper()
	hosts, errorValue := os.ReadFile(computer.hostsPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return string(hosts)
}

func TestAnEmptyBoxIsRenamedAfterTheCompanyThatClaimsIt(t *testing.T) {
	computer := aComputerNamed(t, EmptyBoxHostName)
	computer.nameFor(t, sampleCompanySlug)
	if computer.hostName != sampleCompanySlug {
		t.Fatalf("host name = %q, want %q; calls = %v", computer.hostName, sampleCompanySlug, computer.calls)
	}
	if !containsCall(computer.calls, []string{"systemctl", "try-restart", multicastDNSService}) {
		t.Errorf("calls = %v, want %s restarted after the rename", computer.calls, multicastDNSService)
	}
	if hosts := computer.hosts(t); hosts != "127.0.0.1\tlocalhost\n127.0.1.1\t"+sampleCompanySlug+"\n" {
		t.Errorf("hosts = %q, want 127.0.1.1 to follow the new name", hosts)
	}
}

func TestADisconnectedComputerInternKimNamedIsNamedKimminiAgain(t *testing.T) {
	computer := aComputerNamed(t, EmptyBoxHostName)
	computer.nameFor(t, sampleCompanySlug)
	computer.nameFor(t, "")
	if computer.hostName != EmptyBoxHostName {
		t.Fatalf("host name after disconnecting = %q, want %q", computer.hostName, EmptyBoxHostName)
	}
	computer.nameFor(t, "another-company")
	if computer.hostName != "another-company" {
		t.Fatalf("host name after another company claimed it = %q, want another-company", computer.hostName)
	}
}

func TestAComputerSomeoneNamedKeepsItsName(t *testing.T) {
	for _, hostName := range []string{"internkim", sampleCompanySlug, "kimminis", "kimmini-", "kimmini-a"} {
		computer := aComputerNamed(t, hostName)
		before := computer.hosts(t)
		computer.nameFor(t, sampleCompanySlug)
		computer.nameFor(t, "")
		if len(computer.calls) != 0 || computer.hostName != hostName {
			t.Errorf("a computer named %q became %q; calls = %v", hostName, computer.hostName, computer.calls)
		}
		if after := computer.hosts(t); after != before {
			t.Errorf("hosts on a computer named %q changed to %q", hostName, after)
		}
	}
}

func TestTheNameMulticastDNSNumberedIsAnEmptyBoxName(t *testing.T) {
	computer := aComputerNamed(t, "kimmini-2")
	computer.nameFor(t, sampleCompanySlug)
	if computer.hostName != sampleCompanySlug {
		t.Fatalf("host name = %q, want %q", computer.hostName, sampleCompanySlug)
	}
}

func TestAdoptingTakesTheCurrentNameAsInternKimsOnce(t *testing.T) {
	computer := aComputerNamed(t, sampleCompanySlug)
	if errorValue := computer.namer.AdoptCurrentName(); errorValue != nil {
		t.Fatal(errorValue)
	}
	computer.nameFor(t, "")
	if computer.hostName != EmptyBoxHostName {
		t.Fatalf("an adopted name %q was kept on disconnect as %q", sampleCompanySlug, computer.hostName)
	}
	computer.hostName = "renamed-by-hand"
	if errorValue := computer.namer.AdoptCurrentName(); errorValue != nil {
		t.Fatal(errorValue)
	}
	computer.nameFor(t, sampleCompanySlug)
	if computer.hostName != "renamed-by-hand" {
		t.Fatalf("adopting again took a name someone gave by hand; host name = %q", computer.hostName)
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
