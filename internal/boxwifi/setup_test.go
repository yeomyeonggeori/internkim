package boxwifi

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"path/filepath"
	"reflect"
	"slices"
	"sync"
	"testing"
	"time"
)

type fakeRadio struct {
	mutex sync.Mutex

	online          bool
	networks        []Network
	scanError       error
	openError       error
	address         string
	joinErrors      []error
	onlineAfterJoin bool

	checksUntilOnline int
	onlineChecks      int

	connectingChecksLeft int

	events     []string
	joins      []submission
	scanCalls  int
	openCalls  int
	closeCalls int
	openNames  []string
}

func (radio *fakeRadio) IsOnline(ctx context.Context) bool {
	radio.mutex.Lock()
	defer radio.mutex.Unlock()
	radio.onlineChecks++
	if radio.checksUntilOnline > 0 && radio.onlineChecks >= radio.checksUntilOnline {
		return true
	}
	return radio.online
}

func (radio *fakeRadio) IsConnecting(ctx context.Context) bool {
	radio.mutex.Lock()
	defer radio.mutex.Unlock()
	if radio.connectingChecksLeft == 0 {
		return false
	}
	radio.connectingChecksLeft--
	radio.events = append(radio.events, "connecting")
	return true
}

func (radio *fakeRadio) Scan(ctx context.Context) ([]Network, error) {
	radio.mutex.Lock()
	defer radio.mutex.Unlock()
	radio.scanCalls++
	radio.events = append(radio.events, "scan")
	return radio.networks, radio.scanError
}

func (radio *fakeRadio) OpenSetupNetwork(ctx context.Context, name string) (string, error) {
	radio.mutex.Lock()
	defer radio.mutex.Unlock()
	radio.openCalls++
	radio.openNames = append(radio.openNames, name)
	radio.events = append(radio.events, "open")
	if radio.openError != nil {
		return "", radio.openError
	}
	return radio.address, nil
}

func (radio *fakeRadio) CloseSetupNetwork(ctx context.Context) error {
	radio.mutex.Lock()
	defer radio.mutex.Unlock()
	radio.closeCalls++
	radio.events = append(radio.events, "close")
	return nil
}

func (radio *fakeRadio) Join(ctx context.Context, ssid, password string) error {
	radio.mutex.Lock()
	defer radio.mutex.Unlock()
	radio.events = append(radio.events, "join")
	radio.joins = append(radio.joins, submission{ssid: ssid, password: password})
	var errorValue error
	if len(radio.joinErrors) > 0 {
		errorValue = radio.joinErrors[0]
		radio.joinErrors = radio.joinErrors[1:]
	}
	if errorValue == nil && radio.onlineAfterJoin {
		radio.online = true
	}
	return errorValue
}

func testListener(listenerReady chan<- net.Listener) func(string) (net.Listener, error) {
	return func(string) (net.Listener, error) {
		listener, errorValue := net.Listen("tcp", "127.0.0.1:0")
		if errorValue != nil {
			return nil, errorValue
		}
		listenerReady <- listener
		return listener, nil
	}
}

func submitJoin(t *testing.T, baseURL, ssid, password string) *http.Response {
	t.Helper()
	response, errorValue := http.PostForm(baseURL+"/join", url.Values{"ssid": {ssid}, "password": {password}})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return response
}

func TestSetupSkipsEverythingWhenAlreadyOnline(t *testing.T) {
	radio := &fakeRadio{online: true}
	setup := withFakeClock(Setup{Radio: radio})

	if errorValue := setup.Run(context.Background()); errorValue != nil {
		t.Fatal(errorValue)
	}
	if radio.scanCalls != 0 || radio.openCalls != 0 {
		t.Fatalf("scanCalls = %d, openCalls = %d, want 0 and 0", radio.scanCalls, radio.openCalls)
	}
}

func TestSetupScansOpensClosesBeforeJoinAndReturnsOnceOnline(t *testing.T) {
	radio := &fakeRadio{
		address:         "10.0.0.5",
		onlineAfterJoin: true,
		networks:        []Network{{SSID: "Office", SignalPercent: 80}},
	}
	listenerReady := make(chan net.Listener, 4)
	setup := withFakeClock(Setup{
		Radio:  radio,
		Listen: testListener(listenerReady),
	})
	runErrors := make(chan error, 1)
	go func() { runErrors <- setup.Run(context.Background()) }()

	listener := <-listenerReady
	submitJoin(t, "http://"+listener.Addr().String(), "Office", "office-secret").Body.Close()

	select {
	case errorValue := <-runErrors:
		if errorValue != nil {
			t.Fatal(errorValue)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after joining")
	}

	if radio.scanCalls != 1 || radio.openCalls != 1 || radio.closeCalls != 1 {
		t.Fatalf("scanCalls = %d, openCalls = %d, closeCalls = %d", radio.scanCalls, radio.openCalls, radio.closeCalls)
	}
	if want := []string{"scan", "open", "close", "join"}; !reflect.DeepEqual(radio.events, want) {
		t.Fatalf("events = %v, want %v", radio.events, want)
	}
	if len(radio.joins) != 1 || radio.joins[0].ssid != "Office" || radio.joins[0].password != "office-secret" {
		t.Fatalf("joins = %v", radio.joins)
	}
}

func TestSetupReopensAfterJoinFailureAndShowsTheFailureNotice(t *testing.T) {
	radio := &fakeRadio{
		address:         "10.0.0.5",
		onlineAfterJoin: true,
		joinErrors:      []error{errors.New("secrets were rejected")},
	}
	listenerReady := make(chan net.Listener, 4)
	setup := withFakeClock(Setup{
		Radio:  radio,
		Listen: testListener(listenerReady),
	})
	runErrors := make(chan error, 1)
	go func() { runErrors <- setup.Run(context.Background()) }()

	firstListener := <-listenerReady
	submitJoin(t, "http://"+firstListener.Addr().String(), "Office", "wrong-password").Body.Close()

	secondListener := <-listenerReady
	baseURL := "http://" + secondListener.Addr().String()
	var listing networkListing
	if errorValue := json.Unmarshal([]byte(fetchBody(t, baseURL+"/networks")), &listing); errorValue != nil {
		t.Fatal(errorValue)
	}
	if !listing.HasJoinFailed {
		t.Fatal("the reopened page did not report the failed join")
	}

	submitJoin(t, baseURL, "Office", "correct-password").Body.Close()

	select {
	case errorValue := <-runErrors:
		if errorValue != nil {
			t.Fatal(errorValue)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after the second join")
	}

	if radio.openCalls != 2 {
		t.Fatalf("openCalls = %d, want 2", radio.openCalls)
	}
	if len(radio.joins) != 2 {
		t.Fatalf("joins = %v, want 2 attempts", radio.joins)
	}
}

func TestSetupClosesTheAccessPointWhenTheContextIsCancelledWhileWaiting(t *testing.T) {
	radio := &fakeRadio{address: "10.0.0.5"}
	listenerReady := make(chan net.Listener, 1)
	setup := withFakeClock(Setup{
		Radio:  radio,
		Listen: testListener(listenerReady),
	})
	ctx, cancel := context.WithCancel(context.Background())
	runErrors := make(chan error, 1)
	go func() { runErrors <- setup.Run(ctx) }()

	<-listenerReady
	cancel()

	select {
	case errorValue := <-runErrors:
		if !errors.Is(errorValue, context.Canceled) {
			t.Fatalf("error = %v, want context.Canceled", errorValue)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after cancellation")
	}
	if radio.closeCalls != 1 {
		t.Fatalf("closeCalls = %d, want 1", radio.closeCalls)
	}
}

func TestASavedNetworkThatComesUpDuringStartupKeepsKimminiClosed(t *testing.T) {
	radio := &fakeRadio{checksUntilOnline: 4}
	setup := withFakeClock(Setup{Radio: radio})

	if errorValue := setup.Run(context.Background()); errorValue != nil {
		t.Fatal(errorValue)
	}
	if radio.openCalls != 0 {
		t.Fatalf("the setup network opened %d times while a saved network was still coming up", radio.openCalls)
	}
}

func TestSetupClosesTheAccessPointWhenOpeningItFails(t *testing.T) {
	radio := &fakeRadio{openError: errors.New("no wifi device")}
	setup := withFakeClock(Setup{Radio: radio})

	if errorValue := setup.Run(context.Background()); errorValue == nil {
		t.Fatal("expected an error")
	}
	if radio.closeCalls != 1 {
		t.Fatalf("closeCalls = %d, want 1", radio.closeCalls)
	}
}

func TestSetupClosesTheAccessPointAfterTheWindowAndStopsOnceOnline(t *testing.T) {
	radio := &fakeRadio{address: "10.0.0.5", checksUntilOnline: 3}
	listenerReady := make(chan net.Listener, 4)
	setup := withFakeClock(Setup{
		Radio:                  radio,
		Listen:                 testListener(listenerReady),
		SavedNetworkWaitAtBoot: time.Nanosecond,
		SetupWindow:            20 * time.Millisecond,
	})
	runErrors := make(chan error, 1)
	go func() { runErrors <- setup.Run(context.Background()) }()

	select {
	case errorValue := <-runErrors:
		if errorValue != nil {
			t.Fatal(errorValue)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after the setup window elapsed")
	}
	if radio.closeCalls < 1 || radio.openCalls < 1 {
		t.Fatalf("openCalls = %d, closeCalls = %d", radio.openCalls, radio.closeCalls)
	}
	if len(radio.joins) != 0 {
		t.Fatalf("joins = %v, want none", radio.joins)
	}
}

func TestSetupReopensTheAccessPointWhenTheWindowElapsesWhileStillOffline(t *testing.T) {
	radio := &fakeRadio{address: "10.0.0.5"}
	listenerReady := make(chan net.Listener, 8)
	setup := withFakeClock(Setup{
		Radio:                  radio,
		Listen:                 testListener(listenerReady),
		SavedNetworkWaitAtBoot: time.Nanosecond,
		SetupWindow:            20 * time.Millisecond,
	})
	ctx, cancel := context.WithCancel(context.Background())
	runErrors := make(chan error, 1)
	go func() { runErrors <- setup.Run(ctx) }()

	<-listenerReady
	<-listenerReady
	cancel()
	<-runErrors
	if radio.openCalls < 2 {
		t.Fatalf("openCalls = %d, want the access point reopened", radio.openCalls)
	}
}

func TestSetupWaitsForTheSavedNetworkAfterTheWindowWithoutReopening(t *testing.T) {
	radio := &fakeRadio{address: "10.0.0.5", checksUntilOnline: 8}
	listenerReady := make(chan net.Listener, 8)
	setup := withFakeClock(Setup{
		Radio:                  radio,
		Listen:                 testListener(listenerReady),
		SavedNetworkWaitAtBoot: time.Nanosecond,
		SetupWindow:            20 * time.Millisecond,
	})
	runErrors := make(chan error, 1)
	go func() { runErrors <- setup.Run(context.Background()) }()

	select {
	case errorValue := <-runErrors:
		if errorValue != nil {
			t.Fatal(errorValue)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after the saved network came up")
	}
	if radio.openCalls != 1 {
		t.Fatalf("openCalls = %d, want 1", radio.openCalls)
	}
}

type fakeClock struct {
	current time.Time
}

func withFakeClock(setup Setup) Setup {
	clock := &fakeClock{current: time.Unix(0, 0)}
	setup.Now = func() time.Time { return clock.current }
	setup.Sleep = func(ctx context.Context, wait time.Duration) error {
		clock.current = clock.current.Add(wait)
		return ctx.Err()
	}
	return setup
}

func TestSetupMeasuresTheOnlineWaitByTheClockNotByThePollCount(t *testing.T) {
	radio := &fakeRadio{}
	clock := &fakeClock{current: time.Unix(0, 0)}
	probeDuration := 20 * time.Second
	slowRadio := &slowProbeRadio{fakeRadio: radio, advance: func() { clock.current = clock.current.Add(probeDuration) }}
	setup := Setup{
		Radio: slowRadio,
		Now:   func() time.Time { return clock.current },
		Sleep: func(ctx context.Context, wait time.Duration) error {
			clock.current = clock.current.Add(wait)
			return nil
		},
	}

	if setup.waitUntilOnline(context.Background(), 45*time.Second) {
		t.Fatal("the radio never came online, yet the wait reported success")
	}
	if slowRadio.probes > 3 {
		t.Fatalf("probes = %d, want the 45s wall-clock deadline to stop slow probes after 3", slowRadio.probes)
	}
}

type slowProbeRadio struct {
	*fakeRadio
	advance func()
	probes  int
}

func (radio *slowProbeRadio) IsOnline(ctx context.Context) bool {
	radio.probes++
	radio.advance()
	return false
}

func TestABoxWithACompanyOnANetworkWithoutTheInternetKeepsTheSetupNetworkClosed(t *testing.T) {
	radio := &fakeRadio{address: "10.0.0.5"}
	setup := withFakeClock(Setup{Radio: radio, Listen: testListener(make(chan net.Listener, 4)), IsOnANetwork: func(context.Context) bool { return true }})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if errorValue := setup.Run(ctx); errorValue != nil {
		t.Fatal(errorValue)
	}
	if radio.scanCalls != 0 || radio.openCalls != 0 {
		t.Fatalf("a box still on its network opened the setup network: scanCalls = %d, openCalls = %d", radio.scanCalls, radio.openCalls)
	}
}

func TestABoxWithACompanyOnNoNetworkOpensTheSetupNetwork(t *testing.T) {
	radio := &fakeRadio{address: "10.0.0.5", onlineAfterJoin: true, networks: []Network{{SSID: "Office", SignalPercent: 80}}}
	listenerReady := make(chan net.Listener, 4)
	setup := withFakeClock(Setup{Radio: radio, Listen: testListener(listenerReady), IsOnANetwork: func(context.Context) bool { return false }})
	runErrors := make(chan error, 1)
	go func() { runErrors <- setup.Run(context.Background()) }()

	listener := <-listenerReady
	submitJoin(t, "http://"+listener.Addr().String(), "Office", "office-secret").Body.Close()

	select {
	case errorValue := <-runErrors:
		if errorValue != nil {
			t.Fatal(errorValue)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after joining")
	}
	if radio.openCalls != 1 {
		t.Fatalf("a box with a company on no network opened the setup network %d times, want once", radio.openCalls)
	}
}

func TestTheSetupNetworkWaitsForAConnectionNetworkManagerIsMaking(t *testing.T) {
	radio := &fakeRadio{address: "10.0.0.5", connectingChecksLeft: 3}
	listenerReady := make(chan net.Listener, 4)
	setup := withFakeClock(Setup{Radio: radio, Listen: testListener(listenerReady), SavedNetworkWaitAtBoot: time.Nanosecond})
	ctx, cancel := context.WithCancel(context.Background())
	runErrors := make(chan error, 1)
	go func() { runErrors <- setup.Run(ctx) }()
	<-listenerReady
	cancel()
	<-runErrors

	radio.mutex.Lock()
	defer radio.mutex.Unlock()
	want := []string{"connecting", "connecting", "connecting", "scan", "open"}
	if len(radio.events) < len(want) || !slices.Equal(radio.events[:len(want)], want) {
		t.Fatalf("events = %v, want the setup network opened only after NetworkManager stopped connecting", radio.events)
	}
}

func TestAConnectionNetworkManagerFinishesKeepsTheSetupNetworkClosed(t *testing.T) {
	radio := &fakeRadio{address: "10.0.0.5", connectingChecksLeft: 2, checksUntilOnline: 3}
	setup := withFakeClock(Setup{Radio: radio, Listen: testListener(make(chan net.Listener, 4)), SavedNetworkWaitAtBoot: time.Nanosecond})

	if errorValue := setup.Run(context.Background()); errorValue != nil {
		t.Fatal(errorValue)
	}
	if radio.openCalls != 0 {
		t.Fatalf("the setup network opened %d times over a connection NetworkManager brought online", radio.openCalls)
	}
}

func TestABoxWithACompanyDoesNotReopenTheSetupNetworkOnceANetworkHoldsIt(t *testing.T) {
	radio := &fakeRadio{address: "10.0.0.5"}
	listenerReady := make(chan net.Listener, 8)
	isOnANetwork := false
	var networkMutex sync.Mutex
	setup := withFakeClock(Setup{
		Radio:                  radio,
		Listen:                 testListener(listenerReady),
		SavedNetworkWaitAtBoot: time.Nanosecond,
		SetupWindow:            20 * time.Millisecond,
		IsOnANetwork: func(context.Context) bool {
			networkMutex.Lock()
			defer networkMutex.Unlock()
			return isOnANetwork
		},
	})
	runErrors := make(chan error, 1)
	go func() { runErrors <- setup.Run(context.Background()) }()

	<-listenerReady
	networkMutex.Lock()
	isOnANetwork = true
	networkMutex.Unlock()
	select {
	case errorValue := <-runErrors:
		if errorValue != nil {
			t.Fatal(errorValue)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Run kept reopening the setup network after a saved network without the internet came up")
	}
	if radio.openCalls != 1 {
		t.Fatalf("openCalls = %d, want the setup network left closed once the box joined a network", radio.openCalls)
	}
}

type fakeNetworkManager struct {
	mutex   sync.Mutex
	devices string
	calls   [][]string
}

func (manager *fakeNetworkManager) run(ctx context.Context, name string, arguments ...string) ([]byte, error) {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	manager.calls = append(manager.calls, arguments)
	switch {
	case slices.Equal(arguments, []string{"-t", "-f", "CONNECTIVITY", "general", "status"}):
		return []byte("limited\n"), nil
	case slices.Equal(arguments, []string{"-t", "-f", "TYPE,STATE", "device"}):
		return []byte(manager.devices), nil
	case slices.Contains(arguments, "IP4.ADDRESS"):
		return []byte("10.42.0.1/24\n"), nil
	}
	return nil, nil
}

func (manager *fakeNetworkManager) hasOpenedTheSetupNetwork() bool {
	return manager.openedSetupNetworkName() != ""
}

func (manager *fakeNetworkManager) openedSetupNetworkName() string {
	manager.mutex.Lock()
	defer manager.mutex.Unlock()
	for _, arguments := range manager.calls {
		ssidIndex := slices.Index(arguments, "ssid")
		if slices.Contains(arguments, "add") && slices.Contains(arguments, "ap") && ssidIndex >= 0 && ssidIndex+1 < len(arguments) {
			return arguments[ssidIndex+1]
		}
	}
	return ""
}

func setupForAHostWithACompany(t *testing.T, manager *fakeNetworkManager, listenerReady chan<- net.Listener) Setup {
	radio := NetworkManagerRadio{Run: manager.run, CaptiveDNSPath: filepath.Join(t.TempDir(), "internkim-setup.conf")}
	hostName := func() (string, error) { return "internkim", nil }
	return withFakeClock(Setup{Radio: radio, IsOnANetwork: radio.IsOnANetwork, Listen: testListener(listenerReady), HostName: hostName})
}

func TestAHostWithACompanyOnAWiredNetworkWithoutTheInternetNeverOpensTheSetupNetwork(t *testing.T) {
	manager := &fakeNetworkManager{devices: "wifi:disconnected\nethernet:connected\nloopback:connected (externally)\n"}
	setup := setupForAHostWithACompany(t, manager, make(chan net.Listener, 1))

	if errorValue := setup.Run(context.Background()); errorValue != nil {
		t.Fatal(errorValue)
	}
	if manager.hasOpenedTheSetupNetwork() {
		t.Fatalf("a wired host whose internet is out opened the setup network; nmcli calls = %v", manager.calls)
	}
}

func TestAHostWithACompanyOffEveryNetworkOpensTheSetupNetwork(t *testing.T) {
	manager := &fakeNetworkManager{devices: "wifi:disconnected\nethernet:unavailable\nloopback:connected (externally)\n"}
	listenerReady := make(chan net.Listener, 1)
	setup := setupForAHostWithACompany(t, manager, listenerReady)
	ctx, cancel := context.WithCancel(context.Background())
	runErrors := make(chan error, 1)
	go func() { runErrors <- setup.Run(ctx) }()

	<-listenerReady
	cancel()
	<-runErrors
	if opened := manager.openedSetupNetworkName(); opened != "internkim" {
		t.Fatalf("a host named internkim off every network opened the setup network %q, want internkim; nmcli calls = %v", opened, manager.calls)
	}
}

func TestTheSetupNetworkTakesTheHostNameItHasWhenItOpens(t *testing.T) {
	radio := &fakeRadio{address: "10.0.0.5"}
	listenerReady := make(chan net.Listener, 8)
	var hostNameMutex sync.Mutex
	hostNames := []string{"kimmini", "yeomyeonggeori"}
	setup := withFakeClock(Setup{
		Radio:                  radio,
		Listen:                 testListener(listenerReady),
		SavedNetworkWaitAtBoot: time.Nanosecond,
		SetupWindow:            20 * time.Millisecond,
		MadeOn:                 time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC),
		HostName: func() (string, error) {
			hostNameMutex.Lock()
			defer hostNameMutex.Unlock()
			current := hostNames[0]
			if len(hostNames) > 1 {
				hostNames = hostNames[1:]
			}
			return current, nil
		},
	})
	ctx, cancel := context.WithCancel(context.Background())
	runErrors := make(chan error, 1)
	go func() { runErrors <- setup.Run(ctx) }()

	<-listenerReady
	<-listenerReady
	cancel()
	<-runErrors
	radio.mutex.Lock()
	defer radio.mutex.Unlock()
	if len(radio.openNames) < 2 || radio.openNames[0] != "kimmini-261002" || radio.openNames[1] != "yeomyeonggeori-261002" {
		t.Fatalf("opened %v, want kimmini-261002 and then yeomyeonggeori-261002 as the host name changed", radio.openNames)
	}
}
