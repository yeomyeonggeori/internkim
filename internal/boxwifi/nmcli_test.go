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
	"unicode/utf8"
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

func TestTheSetupNetworkIsNamedAfterTheHostAndTheDayTheBoxWasMade(t *testing.T) {
	madeOn := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		hostName string
		madeOn   time.Time
		want     string
	}{
		{hostName: "kimmini", madeOn: madeOn, want: "kimmini-261002"},
		{hostName: "jetson-host", want: "jetson-host"},
		{hostName: "yeomyeonggeori", madeOn: madeOn, want: "yeomyeonggeori-261002"},
		{hostName: "", want: EmptyBoxHostName},
	}
	for _, testCase := range cases {
		if got := SetupNetworkNameFor(testCase.hostName, testCase.madeOn); got != testCase.want {
			t.Errorf("setup network name for host %q made on %v = %q, want %q", testCase.hostName, testCase.madeOn, got, testCase.want)
		}
	}
}

func TestTheSetupNetworkNameFitsTheWifiLimitAndKeepsItsDate(t *testing.T) {
	madeOn := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
	longHostName := strings.Repeat("a", 63)
	dated := SetupNetworkNameFor(longHostName, madeOn)
	if len(dated) != 32 || !strings.HasSuffix(dated, "-261002") {
		t.Fatalf("setup network name = %q (%d bytes), want 32 bytes ending in -261002", dated, len(dated))
	}
	if undated := SetupNetworkNameFor(longHostName, time.Time{}); undated != strings.Repeat("a", 32) {
		t.Fatalf("setup network name without a day = %q, want the host name cut to 32 bytes", undated)
	}
	multibyte := SetupNetworkNameFor(strings.Repeat("a", 24)+"사무실", madeOn)
	if len(multibyte) > 32 || !utf8.ValidString(multibyte) {
		t.Fatalf("setup network name = %q (%d bytes), want at most 32 bytes of valid UTF-8", multibyte, len(multibyte))
	}
}

func TestOnlyAComputerWhoseWifiNetworkManagerManagesSetsUpWifi(t *testing.T) {
	cases := []struct {
		devices string
		want    bool
	}{
		{devices: "ethernet:connected\nwifi:disconnected\nloopback:connected (externally)\n", want: true},
		{devices: "wifi:unavailable\n", want: true},
		{devices: "ethernet:connected\nloopback:connected (externally)\n", want: false},
		{devices: "wifi:unmanaged\nethernet:connected\n", want: false},
		{devices: "wifi-p2p:disconnected\nethernet:connected\n", want: false},
	}
	for _, testCase := range cases {
		radio := NetworkManagerRadio{Run: func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
			return []byte(testCase.devices), nil
		}}
		if got := radio.ManagesWifi(context.Background()); got != testCase.want {
			t.Errorf("ManagesWifi with devices %q = %t, want %t", testCase.devices, got, testCase.want)
		}
	}
}

func TestADeviceNetworkManagerIsActivatingIsConnectingAndNotYetOnANetwork(t *testing.T) {
	devices := "wifi:connecting (getting IP configuration)\nethernet:unavailable\nloopback:connected (externally)\n"
	radio := NetworkManagerRadio{Run: func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
		return []byte(devices), nil
	}}
	if !radio.IsConnecting(context.Background()) {
		t.Fatalf("devices %q were not judged to be connecting", devices)
	}
	if radio.IsOnANetwork(context.Background()) {
		t.Fatalf("devices %q were judged to be on a network before the connection finished", devices)
	}
	settled := NetworkManagerRadio{Run: func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
		return []byte("wifi:disconnected\nethernet:connected\n"), nil
	}}
	if settled.IsConnecting(context.Background()) {
		t.Fatal("devices that finished connecting were still judged to be connecting")
	}
}

func TestAComputerWithoutNetworkManagerDoesNotSetUpWifi(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	if (NetworkManagerRadio{}).ManagesWifi(context.Background()) {
		t.Fatal("a computer without nmcli was judged to manage a Wi-Fi device")
	}
	failing := NetworkManagerRadio{Run: func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
		return []byte("Error: NetworkManager is not running."), errors.New("exit status 8")
	}}
	if failing.ManagesWifi(context.Background()) {
		t.Fatal("a computer whose NetworkManager is not running was judged to manage a Wi-Fi device")
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

func TestIsOnANetworkCountsAConnectedWifiOrEthernetDeviceOnly(t *testing.T) {
	cases := []struct {
		devices      string
		commandError error
		want         bool
	}{
		{"wifi:connected\nethernet:unavailable\nloopback:connected (externally)\n", nil, true},
		{"wifi:disconnected\nethernet:connected\n", nil, true},
		{"wifi:disconnected\nwifi-p2p:disconnected\nethernet:unavailable\nloopback:connected (externally)\n", nil, false},
		{"wifi:connecting (getting IP configuration)\n", nil, false},
		{"", errors.New("exit status 8"), true},
	}
	for _, testCase := range cases {
		radio := NetworkManagerRadio{Run: func(ctx context.Context, name string, arguments ...string) ([]byte, error) {
			return []byte(testCase.devices), testCase.commandError
		}}
		if got := radio.IsOnANetwork(context.Background()); got != testCase.want {
			t.Fatalf("devices %q, command error %v: IsOnANetwork = %t, want %t", testCase.devices, testCase.commandError, got, testCase.want)
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
