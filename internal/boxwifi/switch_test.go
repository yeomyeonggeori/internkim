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

func TestSwitchJoinsAndRenamesThePendingProfileOnSuccess(t *testing.T) {
	var calls [][]string
	radio := NetworkManagerRadio{KeyfileDirectory: t.TempDir(), Run: func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
		calls = append(calls, append([]string{}, arguments...))
		joined := strings.Join(arguments, " ")
		switch {
		case strings.Contains(joined, "connection show --active"):
			return []byte("Office-Old:802-11-wireless\nWired connection 1:802-3-ethernet\n"), nil
		case strings.Contains(joined, "CONNECTIVITY"):
			return []byte("full"), nil
		default:
			return nil, nil
		}
	}}

	errorValue := radio.Switch(context.Background(), "Office-New", "office-password")

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	wantModify := []string{"connection", "modify", "id", pendingConnectionName, "connection.id", "Office-New"}
	wantDeleteStale := []string{"connection", "delete", "id", "Office-New"}
	foundModify, foundDeleteStale := false, false
	for _, call := range calls {
		if equalStrings(call, wantModify) {
			foundModify = true
		}
		if equalStrings(call, wantDeleteStale) {
			foundDeleteStale = true
		}
	}
	if !foundDeleteStale {
		t.Fatalf("expected a delete of any stale %s profile, calls = %v", "Office-New", calls)
	}
	if !foundModify {
		t.Fatalf("expected the pending profile renamed to Office-New, calls = %v", calls)
	}
}

func TestSwitchFailsBackToThePreviousNetworkWhenJoiningFails(t *testing.T) {
	var calls [][]string
	radio := NetworkManagerRadio{KeyfileDirectory: t.TempDir(), Run: func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
		calls = append(calls, append([]string{}, arguments...))
		joined := strings.Join(arguments, " ")
		switch {
		case strings.Contains(joined, "connection show --active"):
			return []byte("Office-Old:802-11-wireless\n"), nil
		case strings.Contains(joined, "connection up id "+pendingConnectionName):
			return []byte("Error: Secrets were required, but not provided."), errors.New("exit status 4")
		default:
			return nil, nil
		}
	}}

	errorValue := radio.Switch(context.Background(), "Office-New", "super-secret-password")

	if errorValue == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(errorValue.Error(), "super-secret-password") {
		t.Fatalf("error leaked the password: %v", errorValue)
	}
	if !strings.Contains(errorValue.Error(), "Office-New") {
		t.Fatalf("error is missing the ssid: %v", errorValue)
	}
	wantDeletePending := []string{"connection", "delete", "id", pendingConnectionName}
	wantUpPrevious := []string{"connection", "up", "id", "Office-Old"}
	foundDeletePending, foundUpPrevious := false, false
	for _, call := range calls {
		if equalStrings(call, wantDeletePending) {
			foundDeletePending = true
		}
		if equalStrings(call, wantUpPrevious) {
			foundUpPrevious = true
		}
	}
	if !foundDeletePending {
		t.Fatalf("expected the pending profile deleted on failure, calls = %v", calls)
	}
	if !foundUpPrevious {
		t.Fatalf("expected the previous connection brought back up, calls = %v", calls)
	}
}

func TestSwitchWithNoPreviousNetworkBringsNothingBackUp(t *testing.T) {
	var calls [][]string
	radio := NetworkManagerRadio{KeyfileDirectory: t.TempDir(), Run: func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
		calls = append(calls, append([]string{}, arguments...))
		joined := strings.Join(arguments, " ")
		switch {
		case strings.Contains(joined, "connection show --active"):
			return []byte(""), nil
		case strings.Contains(joined, "connection up id "+pendingConnectionName):
			return []byte("Error: no suitable device found"), errors.New("exit status 10")
		default:
			return nil, nil
		}
	}}

	if errorValue := radio.Switch(context.Background(), "Office-New", ""); errorValue == nil {
		t.Fatal("expected an error")
	}
	for _, call := range calls {
		if len(call) >= 2 && call[0] == "connection" && call[1] == "up" {
			t.Fatalf("there was no previous network, yet a connection was brought up: %v", call)
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for index := range a {
		if a[index] != b[index] {
			return false
		}
	}
	return true
}

func recordingRunner(calls *[][]string) func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
	return func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
		*calls = append(*calls, append([]string{}, arguments...))
		if strings.Contains(strings.Join(arguments, " "), "CONNECTIVITY") {
			return []byte("full"), nil
		}
		return nil, nil
	}
}

func assertNoArgumentContains(t *testing.T, calls [][]string, secret string) {
	t.Helper()
	for _, call := range calls {
		for _, argument := range call {
			if strings.Contains(argument, secret) {
				t.Fatalf("a command line carried the secret: %v", call)
			}
		}
	}
}

func TestSwitchWritesAPrivateKeyfileAndNeverPutsThePasswordOnTheCommandLine(t *testing.T) {
	var calls [][]string
	directory := t.TempDir()
	radio := NetworkManagerRadio{KeyfileDirectory: directory, Run: recordingRunner(&calls)}

	if errorValue := radio.Switch(context.Background(), "Office-New", "office-password"); errorValue != nil {
		t.Fatal(errorValue)
	}

	assertNoArgumentContains(t, calls, "office-password")
	matches, errorValue := filepath.Glob(filepath.Join(directory, pendingConnectionName+"-*"+keyfileExtension))
	if errorValue != nil || len(matches) != 1 {
		t.Fatalf("pending keyfiles = %v, %v, want exactly one", matches, errorValue)
	}
	path := matches[0]
	info, errorValue := os.Stat(path)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("keyfile mode = %v, want 0600", info.Mode().Perm())
	}
	content, errorValue := os.ReadFile(path)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, want := range []string{"[connection]\nid=internkim-pending\ntype=wifi\nautoconnect=true", "[wifi]\nmode=infrastructure\nhidden=true\nssid=Office-New", "[wifi-security]\nkey-mgmt=wpa-psk\npsk=office-password", "[ipv4]\nmethod=auto", "[ipv6]\nmethod=auto"} {
		if !strings.Contains(string(content), want) {
			t.Fatalf("keyfile lacks %q:\n%s", want, content)
		}
	}
	if !containsCall(calls, []string{"connection", "load", path}) {
		t.Fatalf("keyfile was not loaded, calls = %v", calls)
	}
}

func TestSwitchRemovesThePendingKeyfileWhenJoiningFails(t *testing.T) {
	directory := t.TempDir()
	radio := NetworkManagerRadio{KeyfileDirectory: directory, Run: func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
		if strings.Contains(strings.Join(arguments, " "), "connection up id "+pendingConnectionName) {
			return nil, errors.New("exit status 4")
		}
		return nil, nil
	}}

	if errorValue := radio.Switch(context.Background(), "Office-New", "office-password"); errorValue == nil {
		t.Fatal("expected an error")
	}
	if _, errorValue := os.Stat(filepath.Join(directory, pendingConnectionName+keyfileExtension)); !os.IsNotExist(errorValue) {
		t.Fatalf("the pending keyfile remains: %v", errorValue)
	}
}

func TestAKeyfileWithoutAPasswordOmitsTheSecuritySection(t *testing.T) {
	var calls [][]string
	directory := t.TempDir()
	radio := NetworkManagerRadio{KeyfileDirectory: directory, JoinRetryWait: time.Millisecond, Run: recordingRunner(&calls)}

	if errorValue := radio.Join(context.Background(), "Open-Cafe", ""); errorValue != nil {
		t.Fatal(errorValue)
	}
	content, errorValue := os.ReadFile(filepath.Join(directory, joinKeyfilePrefix+"Open-Cafe"+keyfileExtension))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if strings.Contains(string(content), "wifi-security") || strings.Contains(string(content), "psk") {
		t.Fatalf("an open network's keyfile has a security section:\n%s", content)
	}
}

func TestJoinNeverPutsThePasswordOnTheCommandLineAndEscapesKeyfileValues(t *testing.T) {
	var calls [][]string
	directory := t.TempDir()
	radio := NetworkManagerRadio{KeyfileDirectory: directory, JoinRetryWait: time.Millisecond, Run: recordingRunner(&calls)}

	if errorValue := radio.Join(context.Background(), ` Back\slash`, "pass\nword"); errorValue != nil {
		t.Fatal(errorValue)
	}

	assertNoArgumentContains(t, calls, "pass")
	content, errorValue := os.ReadFile(filepath.Join(directory, joinKeyfilePrefix+` Back\slash`+keyfileExtension))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	for _, want := range []string{`id=\sBack\\slash`, `ssid=\sBack\\slash`, `psk=pass\nword`} {
		if !strings.Contains(string(content), want) {
			t.Fatalf("keyfile lacks %q:\n%s", want, content)
		}
	}
}

func TestAKeyfileRefusesAnSSIDWithANulByte(t *testing.T) {
	var calls [][]string
	radio := NetworkManagerRadio{KeyfileDirectory: t.TempDir(), Run: recordingRunner(&calls)}

	if errorValue := radio.Join(context.Background(), "Bad\x00Net", "password"); errorValue == nil {
		t.Fatal("expected an error")
	}
	if containsCall(calls, []string{"connection", "load"}) {
		t.Fatalf("a keyfile for a NUL ssid was loaded: %v", calls)
	}
}

func containsCall(calls [][]string, want []string) bool {
	for _, call := range calls {
		if equalStrings(call, want) {
			return true
		}
	}
	return false
}

func TestSwitchNeverOverwritesTheFileOfTheNetworkItIsLeaving(t *testing.T) {
	var calls [][]string
	directory := t.TempDir()
	livePath := filepath.Join(directory, pendingConnectionName+keyfileExtension)
	if errorValue := os.WriteFile(livePath, []byte("the profile in use"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	radio := NetworkManagerRadio{KeyfileDirectory: directory, Run: recordingRunner(&calls)}

	if errorValue := radio.Switch(context.Background(), "Office-New", "office-password"); errorValue != nil {
		t.Fatal(errorValue)
	}

	content, errorValue := os.ReadFile(livePath)
	if errorValue != nil || string(content) != "the profile in use" {
		t.Fatalf("the file of the network in use was replaced: %q, %v", content, errorValue)
	}
}

func TestSwitchDoesNotBringThePreviousNetworkUpWhenTheProfileFailsToLoad(t *testing.T) {
	var calls [][]string
	directory := t.TempDir()
	radio := NetworkManagerRadio{KeyfileDirectory: directory, Run: func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
		calls = append(calls, append([]string{}, arguments...))
		joined := strings.Join(arguments, " ")
		switch {
		case strings.Contains(joined, "connection show --active"):
			return []byte("Office-Old:802-11-wireless\n"), nil
		case strings.Contains(joined, "connection load"):
			return []byte("Error: invalid keyfile"), errors.New("exit status 1")
		default:
			return nil, nil
		}
	}}

	if errorValue := radio.Switch(context.Background(), "Office-New", "pw"); errorValue == nil {
		t.Fatal("expected an error")
	}
	foundDeletePending := false
	for _, call := range calls {
		if len(call) >= 2 && call[0] == "connection" && call[1] == "up" {
			t.Fatalf("nothing was activated, yet a connection was brought up: %v", call)
		}
		if equalStrings(call, []string{"connection", "delete", "id", pendingConnectionName}) {
			foundDeletePending = true
		}
	}
	if !foundDeletePending {
		t.Fatalf("expected the pending profile deleted, calls = %v", calls)
	}
	entries, errorValue := os.ReadDir(directory)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(entries) != 0 {
		t.Fatalf("keyfiles left behind: %v", entries)
	}
}

func TestAKeyfileEscapesASemicolonInTheSSIDOnly(t *testing.T) {
	keyfile := renderKeyfile("a;b", "Cafe;Net", "pass;word")

	for _, want := range []string{`id=a;b`, `ssid=Cafe\\;Net`, `psk=pass;word`} {
		if !strings.Contains(keyfile, want) {
			t.Fatalf("keyfile lacks %q:\n%s", want, keyfile)
		}
	}
}

func switchRunnerWithConnectivity(calls *[][]string, connectivity string) func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
	return func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
		*calls = append(*calls, append([]string{}, arguments...))
		joined := strings.Join(arguments, " ")
		switch {
		case strings.Contains(joined, "connection show --active"):
			return []byte("Office-Old:802-11-wireless\n"), nil
		case strings.Contains(joined, "CONNECTIVITY"):
			return []byte(connectivity), nil
		default:
			return nil, nil
		}
	}
}

func TestSwitchWaitsUntilThePlaneIsReachableAfterConnectionUp(t *testing.T) {
	var calls [][]string
	probes := 0
	clock := time.Unix(0, 0)
	radio := NetworkManagerRadio{
		KeyfileDirectory: t.TempDir(),
		Run:              switchRunnerWithConnectivity(&calls, "full"),
		ReachesPlane:     func(context.Context) bool { probes++; return probes >= 3 },
		Now:              func() time.Time { return clock },
		Sleep: func(ctx context.Context, wait time.Duration) error {
			clock = clock.Add(wait)
			return nil
		},
	}

	if errorValue := radio.Switch(context.Background(), "Office-New", "office-password"); errorValue != nil {
		t.Fatal(errorValue)
	}
	if probes != 3 {
		t.Fatalf("plane probes = %d, want 3", probes)
	}
}

func TestSwitchRevertsWhenThePlaneStaysUnreachableEvenIfNetworkManagerSaysFull(t *testing.T) {
	var calls [][]string
	clock := time.Unix(0, 0)
	radio := NetworkManagerRadio{
		KeyfileDirectory: t.TempDir(),
		Run:              switchRunnerWithConnectivity(&calls, "full"),
		ReachesPlane:     func(context.Context) bool { return false },
		Now:              func() time.Time { return clock },
		Sleep: func(ctx context.Context, wait time.Duration) error {
			clock = clock.Add(wait)
			return nil
		},
	}

	errorValue := radio.Switch(context.Background(), "Office-New", "office-password")

	if errorValue == nil {
		t.Fatal("expected an error")
	}
	if !containsCall(calls, []string{"connection", "up", "id", "Office-Old"}) {
		t.Fatalf("expected the previous connection brought back up, calls = %v", calls)
	}
}

func TestSwitchWithoutAPlaneProbeKeepsWaitingOnNetworkManagerConnectivity(t *testing.T) {
	var calls [][]string
	clock := time.Unix(0, 0)
	radio := NetworkManagerRadio{
		KeyfileDirectory: t.TempDir(),
		Run:              switchRunnerWithConnectivity(&calls, "limited"),
		Now:              func() time.Time { return clock },
		Sleep: func(ctx context.Context, wait time.Duration) error {
			clock = clock.Add(wait)
			return nil
		},
	}

	if errorValue := radio.Switch(context.Background(), "Office-New", "office-password"); errorValue == nil {
		t.Fatal("expected an error while connectivity stays limited")
	}
}

func TestSwitchStillRevertsWhenItsContextIsCancelledDuringTheJoin(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var calls [][]string
	radio := NetworkManagerRadio{KeyfileDirectory: t.TempDir(), Run: func(callContext context.Context, name string, arguments ...string) ([]byte, error) {
		if callContext.Err() != nil {
			return nil, callContext.Err()
		}
		calls = append(calls, append([]string{}, arguments...))
		joined := strings.Join(arguments, " ")
		switch {
		case strings.Contains(joined, "connection show --active"):
			return []byte("Office-Old:802-11-wireless\n"), nil
		case strings.Contains(joined, "connection up id "+pendingConnectionName):
			cancel()
			return nil, errors.New("interrupted")
		default:
			return nil, nil
		}
	}}

	radio.Switch(ctx, "Office-New", "office-password")

	if !containsCall(calls, []string{"connection", "up", "id", "Office-Old"}) {
		t.Fatalf("expected the previous network brought back up after cancellation, calls = %v", calls)
	}
}
