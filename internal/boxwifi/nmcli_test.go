package boxwifi

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestScanParsesEscapedColonsDedupesAndSortsBySignal(t *testing.T) {
	output := "My\\:Net:70:WPA2\nGuest:40:\nGuest:55:WPA2\nMy\\:Net:60:WPA2\n"
	radio := NetworkManagerRadio{Run: func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
		return []byte(output), nil
	}}

	networks, errorValue := radio.Scan(context.Background())
	if errorValue != nil {
		t.Fatal(errorValue)
	}

	want := []Network{
		{SSID: "My:Net", SignalPercent: 70, IsSecured: true},
		{SSID: "Guest", SignalPercent: 55, IsSecured: true},
	}
	if !reflect.DeepEqual(networks, want) {
		t.Fatalf("networks = %+v, want %+v", networks, want)
	}
}

func TestOpenSetupNetworkFallsBackToWpaPskWhenTheFirstUpFails(t *testing.T) {
	var calls [][]string
	upAttempts := 0
	captiveDNSPath := filepath.Join(t.TempDir(), "dnsmasq-shared.d", "internkim-setup.conf")
	radio := NetworkManagerRadio{CaptiveDNSPath: captiveDNSPath, Run: func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
		calls = append(calls, append([]string{}, arguments...))
		if len(arguments) >= 2 && arguments[0] == "--wait" {
			arguments = arguments[2:]
		}
		switch {
		case len(arguments) >= 2 && arguments[0] == "connection" && arguments[1] == "up":
			upAttempts++
			if upAttempts == 1 {
				return []byte("Error: sae key management failed"), errors.New("exit status 1")
			}
			return nil, nil
		case len(arguments) >= 2 && arguments[0] == "-g" && arguments[1] == "IP4.ADDRESS":
			return []byte("10.42.0.1/24\n"), nil
		default:
			return nil, nil
		}
	}}

	address, errorValue := radio.OpenSetupNetwork(context.Background(), "kimmini-rYuA")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if address != "10.42.0.1" {
		t.Fatalf("address = %q, want 10.42.0.1", address)
	}
	written, errorValue := os.ReadFile(captiveDNSPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if string(written) != "address=/#/10.42.0.1\n" {
		t.Fatalf("captive DNS configuration = %q", written)
	}
	if upAttempts != 2 {
		t.Fatalf("up attempts = %d, want 2", upAttempts)
	}

	foundModify := false
	for _, call := range calls {
		if equalStrings(call, []string{"connection", "modify", "id", setupConnectionName, "wifi-sec.key-mgmt", "wpa-psk"}) {
			foundModify = true
		}
	}
	if !foundModify {
		t.Fatalf("expected a fallback connection modify call, calls = %v", calls)
	}
}

func TestJoinErrorNeverContainsThePassword(t *testing.T) {
	radio := NetworkManagerRadio{KeyfileDirectory: t.TempDir(), Run: func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
		return []byte("Error: Secrets were required, but not provided."), errors.New("exit status 4")
	}}

	errorValue := radio.Join(context.Background(), "Office", "super-secret-password")
	if errorValue == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(errorValue.Error(), "super-secret-password") {
		t.Fatalf("error leaked the password: %v", errorValue)
	}
	if !strings.Contains(errorValue.Error(), "Office") {
		t.Fatalf("error is missing the ssid: %v", errorValue)
	}
}

func steppingClock(radio NetworkManagerRadio) NetworkManagerRadio {
	current := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	radio.Now = func() time.Time { return current }
	radio.Sleep = func(ctx context.Context, wait time.Duration) error {
		current = current.Add(wait)
		return nil
	}
	return radio
}

func TestJoinConnectsOnlyOnceTheNetworkAppearsInAScan(t *testing.T) {
	var calls [][]string
	scans := 0
	radio := steppingClock(NetworkManagerRadio{KeyfileDirectory: t.TempDir(), Run: func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
		calls = append(calls, append([]string{}, arguments...))
		if strings.Contains(strings.Join(arguments, " "), "device wifi list") {
			scans++
			if scans < 3 {
				return []byte("Neighbour\n"), nil
			}
			return []byte("Neighbour\nOffice\\:5G\n"), nil
		}
		return nil, nil
	}})

	if errorValue := radio.Join(context.Background(), "Office:5G", "office-password"); errorValue != nil {
		t.Fatal(errorValue)
	}
	if scans != 3 {
		t.Fatalf("scans = %d, want 3", scans)
	}
	ups := 0
	for _, call := range calls {
		if containsCall([][]string{call}, []string{"--wait", "30", "connection", "up", "id", "Office:5G"}) {
			ups++
		}
	}
	if ups != 1 {
		t.Fatalf("connection up ran %d times, want once after the network appeared; calls = %v", ups, calls)
	}
}

func TestJoinStillTriesAHiddenNetworkThatNeverAppearsInAScan(t *testing.T) {
	var calls [][]string
	radio := steppingClock(NetworkManagerRadio{KeyfileDirectory: t.TempDir(), Run: recordingRunner(&calls)})

	if errorValue := radio.Join(context.Background(), "Hidden Office", "office-password"); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !containsCall(calls, []string{"--wait", "30", "connection", "up", "id", "Hidden Office"}) {
		t.Fatalf("expected a connection attempt after the scan wait, calls = %v", calls)
	}
}

func TestEachBoxBroadcastsItsOwnSetupNetworkName(t *testing.T) {
	if got := SetupNetworkNameFor("DfNjfpwsVUGbJ9TutwdZmKHzIFi7_JfUwHtI4uxrYuA"); got != "kimmini-rYuA" {
		t.Fatalf("setup network name = %q, want kimmini-rYuA, the same ending the setup page shows", got)
	}
	if got := SetupNetworkNameFor("ab"); got != SetupNetworkName {
		t.Fatalf("setup network name for a short key = %q, want %q", got, SetupNetworkName)
	}
}

func TestScanMarksTheNetworkInUseEvenWhenAStrongerAccessPointSharesItsName(t *testing.T) {
	output := "Office:80:WPA2: \nOffice:60:WPA2:*\nGuest:90:--: \n"
	networks := parseScanOutput([]byte(output))
	connected := map[string]bool{}
	for _, network := range networks {
		connected[network.SSID] = network.IsConnected
	}
	if !connected["Office"] || connected["Guest"] {
		t.Fatalf("connected = %v, want only Office", connected)
	}
}

func TestCloseSetupNetworkDeletesTheProfileAndFileEvenWhenDownFails(t *testing.T) {
	captiveDNSPath := filepath.Join(t.TempDir(), "internkim-setup.conf")
	if errorValue := os.WriteFile(captiveDNSPath, []byte("address=/#/10.42.0.1\n"), 0o644); errorValue != nil {
		t.Fatal(errorValue)
	}
	var calls [][]string
	radio := NetworkManagerRadio{CaptiveDNSPath: captiveDNSPath, Run: func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
		calls = append(calls, append([]string{}, arguments...))
		if len(arguments) >= 2 && arguments[1] == "down" {
			return nil, errors.New("exit status 10")
		}
		return nil, nil
	}}

	errorValue := radio.CloseSetupNetwork(context.Background())

	if errorValue == nil || !strings.Contains(errorValue.Error(), "closing the") {
		t.Fatalf("error = %v, want the failed down reported", errorValue)
	}
	foundDelete := false
	for _, call := range calls {
		if equalStrings(call, []string{"connection", "delete", "id", setupConnectionName}) {
			foundDelete = true
		}
	}
	if !foundDelete {
		t.Fatalf("delete never ran, calls = %v", calls)
	}
	if _, statError := os.Stat(captiveDNSPath); !os.IsNotExist(statError) {
		t.Fatalf("captive DNS file still present: %v", statError)
	}
}

func TestConnectionsNamedByAnSSIDCarryTheIdKeyword(t *testing.T) {
	for _, ssid := range []string{"3", "help"} {
		var calls [][]string
		radio := steppingClock(NetworkManagerRadio{KeyfileDirectory: t.TempDir(), Run: recordingRunner(&calls)})

		if errorValue := radio.Join(context.Background(), ssid, "office-password"); errorValue != nil {
			t.Fatal(errorValue)
		}
		if !containsCall(calls, []string{"connection", "delete", "id", ssid}) {
			t.Fatalf("ssid %q: delete does not carry id, calls = %v", ssid, calls)
		}
		if !containsCall(calls, []string{"--wait", "30", "connection", "up", "id", ssid}) {
			t.Fatalf("ssid %q: up does not carry id, calls = %v", ssid, calls)
		}

		calls = nil
		if errorValue := radio.Switch(context.Background(), ssid, "office-password"); errorValue != nil {
			t.Fatal(errorValue)
		}
		if !containsCall(calls, []string{"connection", "delete", "id", ssid}) {
			t.Fatalf("ssid %q: switch delete does not carry id, calls = %v", ssid, calls)
		}
		if !containsCall(calls, []string{"connection", "modify", "id", pendingConnectionName, "connection.id", ssid}) {
			t.Fatalf("ssid %q: switch modify does not carry id, calls = %v", ssid, calls)
		}
	}
}

func TestIsOnlineTrustsFullAndOtherwiseAsksThePlane(t *testing.T) {
	reachable := func(context.Context) bool { return true }
	unreachable := func(context.Context) bool { return false }
	cases := []struct {
		connectivity string
		commandError error
		reaches      func(context.Context) bool
		want         bool
	}{
		{"full", nil, nil, true},
		{"full", nil, unreachable, true},
		{"limited", nil, reachable, true},
		{"limited", nil, unreachable, false},
		{"portal", nil, reachable, true},
		{"portal", nil, nil, false},
		{"unknown", nil, reachable, true},
		{"none", nil, reachable, true},
		{"none", nil, unreachable, false},
		{"none", nil, nil, false},
		{"", errors.New("exit status 8"), reachable, true},
		{"", errors.New("exit status 8"), nil, false},
	}
	for _, testCase := range cases {
		radio := NetworkManagerRadio{ReachesPlane: testCase.reaches, Run: func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
			return []byte(testCase.connectivity + "\n"), testCase.commandError
		}}
		if got := radio.IsOnline(context.Background()); got != testCase.want {
			t.Fatalf("connectivity %q, command error %v: IsOnline = %t, want %t", testCase.connectivity, testCase.commandError, got, testCase.want)
		}
	}
}

func TestJoinWritesAKeyfileThatIsNotHiddenForADottedSSID(t *testing.T) {
	var calls [][]string
	directory := t.TempDir()
	radio := steppingClock(NetworkManagerRadio{KeyfileDirectory: directory, Run: recordingRunner(&calls)})

	if errorValue := radio.Join(context.Background(), ".hidden", "office-password"); errorValue != nil {
		t.Fatal(errorValue)
	}
	path := filepath.Join(directory, "internkim-.hidden"+keyfileExtension)
	if _, statError := os.Stat(path); statError != nil {
		t.Fatalf("keyfile %s missing: %v", path, statError)
	}
	if !containsCall(calls, []string{"connection", "load", path}) {
		t.Fatalf("keyfile was not loaded, calls = %v", calls)
	}
}

func TestScanPassesTheRescanModeAndDefaultsToAuto(t *testing.T) {
	for _, testCase := range []struct{ mode, want string }{{"", "auto"}, {"no", "no"}} {
		var calls [][]string
		radio := NetworkManagerRadio{RescanMode: testCase.mode, Run: recordingRunner(&calls)}

		if _, errorValue := radio.Scan(context.Background()); errorValue != nil {
			t.Fatal(errorValue)
		}
		want := []string{"-t", "-f", "SSID,SIGNAL,SECURITY,IN-USE", "device", "wifi", "list", "--rescan", testCase.want}
		if !containsCall(calls, want) {
			t.Fatalf("mode %q: calls = %v, want %v", testCase.mode, calls, want)
		}
	}
}
