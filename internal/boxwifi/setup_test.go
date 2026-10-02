package boxwifi

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/url"
	"reflect"
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

	events     []string
	joins      []submission
	scanCalls  int
	openCalls  int
	closeCalls int
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
