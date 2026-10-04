package box

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

type reportedWifiChange struct {
	requestID string
	result    string
}

type fakeWifiPlane struct {
	pending      *PendingWifiChange
	unclaimed    bool
	reports      []reportedWifiChange
	wifiRequests int
	wifiBodies   [][]byte
}

func (plane *fakeWifiPlane) serve(t *testing.T) *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/box/wifi":
			plane.wifiRequests++
			body, _ := io.ReadAll(request.Body)
			plane.wifiBodies = append(plane.wifiBodies, body)
			if plane.unclaimed {
				writer.WriteHeader(http.StatusNotFound)
				return
			}
			json.NewEncoder(writer).Encode(map[string]any{"companyID": sampleCompanyID, "change": plane.pending})
		case "/api/box/session":
			writer.WriteHeader(http.StatusServiceUnavailable)
		case "/api/box/wifi/outcome":
			var body struct {
				RequestID string `json:"requestID"`
				Result    string `json:"result"`
			}
			json.NewDecoder(request.Body).Decode(&body)
			plane.reports = append(plane.reports, reportedWifiChange{requestID: body.RequestID, result: body.Result})
			writer.WriteHeader(http.StatusOK)
		default:
			t.Errorf("unexpected request to %s", request.URL.Path)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func wifiTestPlaces(t *testing.T, companyID string) Places {
	t.Helper()
	root := t.TempDir()
	places := Places{StateDirectoryPath: filepath.Join(root, "box")}
	if companyID == "" {
		return places
	}
	if errorValue := os.MkdirAll(places.StateDirectoryPath, 0o700); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := os.WriteFile(filepath.Join(places.StateDirectoryPath, companyMarkerFileName), []byte(companyID+"\n"), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	return places
}

func TestAPendingWifiChangeIsOpenedAppliedAndReportedAsJoined(t *testing.T) {
	identity := freshTestIdentity(t)
	plane := &fakeWifiPlane{}
	purpose := WifiNetworkPurpose(sampleCompanyID, identity.EncryptionPublicKey(), "request-1")
	sealed := sealSecretTo(t, identity, `{"ssid":"Office","password":"office-password"}`, purpose)
	plane.pending = &PendingWifiChange{RequestID: "request-1", Sealed: sealed}

	var gotSSID, gotPassword string
	daemon := Daemon{
		Client: Client{AppURL: plane.serve(t).URL},
		Places: wifiTestPlaces(t, sampleCompanyID),
		ChangeWifi: func(ctx context.Context, ssid, password string) error {
			gotSSID, gotPassword = ssid, password
			return nil
		},
	}

	handledRequestID, handledResult := daemon.checkForWifiChange(context.Background(), identity, "", "")

	if gotSSID != "Office" || gotPassword != "office-password" {
		t.Fatalf("ChangeWifi got ssid %q password %q", gotSSID, gotPassword)
	}
	if handledRequestID != "request-1" || handledResult != WifiChangeJoined {
		t.Fatalf("handled %q as %q", handledRequestID, handledResult)
	}
	if len(plane.reports) != 1 || plane.reports[0] != (reportedWifiChange{requestID: "request-1", result: "joined"}) {
		t.Fatalf("reported %+v", plane.reports)
	}
}

func TestAFailingChangeWifiIsReportedAsFailed(t *testing.T) {
	identity := freshTestIdentity(t)
	plane := &fakeWifiPlane{}
	purpose := WifiNetworkPurpose(sampleCompanyID, identity.EncryptionPublicKey(), "request-1")
	plane.pending = &PendingWifiChange{
		RequestID: "request-1",
		Sealed:    sealSecretTo(t, identity, `{"ssid":"Office","password":"office-password"}`, purpose),
	}
	daemon := Daemon{
		Client: Client{AppURL: plane.serve(t).URL},
		Places: wifiTestPlaces(t, sampleCompanyID),
		ChangeWifi: func(ctx context.Context, ssid, password string) error {
			return errors.New("the office network refused the new password")
		},
	}

	handledRequestID, handledResult := daemon.checkForWifiChange(context.Background(), identity, "", "")

	if handledRequestID != "request-1" || handledResult != WifiChangeFailed {
		t.Fatalf("handled %q as %q", handledRequestID, handledResult)
	}
	if len(plane.reports) != 1 || plane.reports[0] != (reportedWifiChange{requestID: "request-1", result: "failed"}) {
		t.Fatalf("reported %+v", plane.reports)
	}
}

func TestABoxThatBelongsToNoCompanyChangesNothing(t *testing.T) {
	plane := &fakeWifiPlane{unclaimed: true}
	daemon := Daemon{
		Client: Client{AppURL: plane.serve(t).URL},
		Places: wifiTestPlaces(t, ""),
		ChangeWifi: func(ctx context.Context, ssid, password string) error {
			t.Fatal("a box that belongs to no company must not change its Wi-Fi")
			return nil
		},
	}

	daemon.checkForWifiChange(context.Background(), freshTestIdentity(t), "", "")

	if len(plane.reports) != 0 {
		t.Fatalf("a box that belongs to no company reported %d outcomes", len(plane.reports))
	}
}

func TestAnAlreadyHandledRequestOnlyRetriesTheReport(t *testing.T) {
	identity := freshTestIdentity(t)
	plane := &fakeWifiPlane{}
	purpose := WifiNetworkPurpose(sampleCompanyID, identity.EncryptionPublicKey(), "request-1")
	plane.pending = &PendingWifiChange{
		RequestID: "request-1",
		Sealed:    sealSecretTo(t, identity, `{"ssid":"Office","password":"office-password"}`, purpose),
	}
	changeWifiCalls := 0
	daemon := Daemon{
		Client: Client{AppURL: plane.serve(t).URL},
		Places: wifiTestPlaces(t, sampleCompanyID),
		ChangeWifi: func(ctx context.Context, ssid, password string) error {
			changeWifiCalls++
			return nil
		},
	}

	handledRequestID, handledResult := daemon.checkForWifiChange(context.Background(), identity, "", "")
	handledRequestID, handledResult = daemon.checkForWifiChange(context.Background(), identity, handledRequestID, handledResult)

	if changeWifiCalls != 1 {
		t.Fatalf("ChangeWifi ran %d times for the same request", changeWifiCalls)
	}
	if len(plane.reports) != 2 {
		t.Fatalf("the outcome was reported %d times, want a retry", len(plane.reports))
	}
	if handledRequestID != "request-1" || handledResult != WifiChangeJoined {
		t.Fatalf("handled %q as %q", handledRequestID, handledResult)
	}
}

func TestWatchForWifiChangesAppliesAPendingChangeOnEachTick(t *testing.T) {
	identity := freshTestIdentity(t)
	plane := &fakeWifiPlane{}
	purpose := WifiNetworkPurpose(sampleCompanyID, identity.EncryptionPublicKey(), "request-1")
	plane.pending = &PendingWifiChange{
		RequestID: "request-1",
		Sealed:    sealSecretTo(t, identity, `{"ssid":"Office","password":"office-password"}`, purpose),
	}
	applied := make(chan string, 1)
	ticks := make(chan struct{})
	daemon := Daemon{
		Client: Client{AppURL: plane.serve(t).URL},
		Places: wifiTestPlaces(t, sampleCompanyID),
		ChangeWifi: func(ctx context.Context, ssid, password string) error {
			applied <- ssid
			return nil
		},
		WatcherSleep: func(ctx context.Context, wait time.Duration) error {
			select {
			case <-ticks:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go daemon.watchForWifiChanges(ctx, identity)

	ticks <- struct{}{}

	select {
	case ssid := <-applied:
		if ssid != "Office" {
			t.Fatalf("applied ssid = %q", ssid)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the watcher never applied the pending change")
	}
}

type wifiNetworkFixture struct {
	CompanyID    string       `json:"companyID"`
	RequestID    string       `json:"requestID"`
	SSID         string       `json:"ssid"`
	Password     string       `json:"password"`
	BoxSecretKey string       `json:"boxSecretKey"`
	Sealed       SealedSecret `json:"sealed"`
}

func TestBoxOpensTheWifiNetworkTheWebWorkerSealed(t *testing.T) {
	document, errorValue := os.ReadFile(filepath.Join("testdata", "sealed-wifi-network.json"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var fixture wifiNetworkFixture
	if errorValue := json.Unmarshal(document, &fixture); errorValue != nil {
		t.Fatal(errorValue)
	}

	identity := identityWithEncryptionSeed(t, fixture.BoxSecretKey)
	purpose := WifiNetworkPurpose(fixture.CompanyID, identity.EncryptionPublicKey(), fixture.RequestID)
	opened, errorValue := identity.OpenSecret(fixture.Sealed, purpose)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	var network struct {
		SSID     string `json:"ssid"`
		Password string `json:"password"`
	}
	if errorValue := json.Unmarshal([]byte(opened), &network); errorValue != nil {
		t.Fatal(errorValue)
	}
	if network.SSID != fixture.SSID || network.Password != fixture.Password {
		t.Fatalf("opened %+v, want ssid %q password %q", network, fixture.SSID, fixture.Password)
	}
}

func pendingChangeForNearbyNetworkTests(t *testing.T, identity Identity) *PendingWifiChange {
	t.Helper()
	purpose := WifiNetworkPurpose(sampleCompanyID, identity.EncryptionPublicKey(), "request-1")
	return &PendingWifiChange{
		RequestID: "request-1",
		Sealed:    sealSecretTo(t, identity, `{"ssid":"Office","password":"office-password"}`, purpose),
	}
}

func TestNearbyNetworksAreSentStrongestFirstCappedAndWithoutOverLongNames(t *testing.T) {
	identity := freshTestIdentity(t)
	plane := &fakeWifiPlane{pending: pendingChangeForNearbyNetworkTests(t, identity)}
	scanned := []NearbyNetwork{
		{SSID: "", SignalPercent: 99},
		{SSID: strings.Repeat("a", 33), SignalPercent: 98},
		{SSID: "Weak", SignalPercent: 10},
	}
	for index := 0; index < 60; index++ {
		scanned = append(scanned, NearbyNetwork{SSID: fmt.Sprintf("Sample-%02d", index), SignalPercent: 20 + index/2, IsSecured: true})
	}
	daemon := Daemon{
		Client: Client{AppURL: plane.serve(t).URL},
		Places: wifiTestPlaces(t, sampleCompanyID),
		ChangeWifi: func(ctx context.Context, ssid, password string) error {
			return nil
		},
		ScanWifi: func(ctx context.Context) ([]NearbyNetwork, error) {
			return scanned, nil
		},
	}

	daemon.checkForWifiChange(context.Background(), identity, "", "")

	var sent struct {
		NearbyNetworks []NearbyNetwork `json:"nearbyNetworks"`
	}
	if errorValue := json.Unmarshal(plane.wifiBodies[0], &sent); errorValue != nil {
		t.Fatalf("body %q: %v", plane.wifiBodies[0], errorValue)
	}
	if len(sent.NearbyNetworks) != 50 {
		t.Fatalf("sent %d networks, want 50", len(sent.NearbyNetworks))
	}
	for index, network := range sent.NearbyNetworks {
		if network.SSID == "" || len(network.SSID) > 32 {
			t.Fatalf("sent unusable ssid %q", network.SSID)
		}
		if index > 0 && network.SignalPercent > sent.NearbyNetworks[index-1].SignalPercent {
			t.Fatalf("network %d is stronger than the one before it", index)
		}
	}
	if !sent.NearbyNetworks[0].IsSecured {
		t.Fatalf("isSecured was not carried: %+v", sent.NearbyNetworks[0])
	}
}

func TestAFailingScanSendsNoBodyAndStillFetchesTheChange(t *testing.T) {
	identity := freshTestIdentity(t)
	plane := &fakeWifiPlane{pending: pendingChangeForNearbyNetworkTests(t, identity)}
	joined := false
	daemon := Daemon{
		Client: Client{AppURL: plane.serve(t).URL},
		Places: wifiTestPlaces(t, sampleCompanyID),
		ChangeWifi: func(ctx context.Context, ssid, password string) error {
			joined = true
			return nil
		},
		ScanWifi: func(ctx context.Context) ([]NearbyNetwork, error) {
			return nil, errors.New("the radio is busy")
		},
	}

	daemon.checkForWifiChange(context.Background(), identity, "", "")

	if len(plane.wifiBodies) != 1 || len(plane.wifiBodies[0]) != 0 {
		t.Fatalf("bodies %q, want one empty body", plane.wifiBodies)
	}
	if !joined {
		t.Fatal("a failing scan blocked the Wi-Fi change")
	}
}

func TestReportableNetworksKeepOnlyTheStrongestOfEachSSIDBeforeCapping(t *testing.T) {
	scanned := []NearbyNetwork{
		{SSID: "Sample-Office", SignalPercent: 30},
		{SSID: "Sample-Office", SignalPercent: 80, IsSecured: true},
		{SSID: "Sample-Guest", SignalPercent: 50},
		{SSID: "Sample-Office", SignalPercent: 60},
	}
	for index := 0; index < 60; index++ {
		scanned = append(scanned, NearbyNetwork{SSID: fmt.Sprintf("Filler-%02d", index), SignalPercent: 1})
	}

	reportable := reportableNetworks(scanned)

	if len(reportable) != 50 {
		t.Fatalf("reported %d networks, want 50", len(reportable))
	}
	if reportable[0].SSID != "Sample-Office" || reportable[0].SignalPercent != 80 || !reportable[0].IsSecured {
		t.Fatalf("first = %+v, want the strongest Sample-Office", reportable[0])
	}
	if reportable[1].SSID != "Sample-Guest" {
		t.Fatalf("second = %+v, want Sample-Guest", reportable[1])
	}
	seen := map[string]bool{}
	for _, network := range reportable {
		if seen[network.SSID] {
			t.Fatalf("ssid %q reported twice", network.SSID)
		}
		seen[network.SSID] = true
	}
}

func TestRunWaitsForAWifiChangeInFlightBeforeReturning(t *testing.T) {
	places := wifiTestPlaces(t, sampleCompanyID)
	identity, errorValue := LoadOrCreateIdentity(identityPathIn(places.StateDirectoryPath))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	plane := &fakeWifiPlane{}
	purpose := WifiNetworkPurpose(sampleCompanyID, identity.EncryptionPublicKey(), "request-1")
	plane.pending = &PendingWifiChange{
		RequestID: "request-1",
		Sealed:    sealSecretTo(t, identity, `{"ssid":"Office","password":"office-password"}`, purpose),
	}
	started := make(chan struct{})
	release := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	daemon := Daemon{
		Client: Client{AppURL: plane.serve(t).URL},
		Places: places,
		ChangeWifi: func(ctx context.Context, ssid, password string) error {
			close(started)
			<-release
			return nil
		},
		WatcherSleep: func(ctx context.Context, wait time.Duration) error {
			return ctx.Err()
		},
		Sleep: func(ctx context.Context, wait time.Duration) error {
			select {
			case <-started:
			case <-time.After(2 * time.Second):
			}
			cancel()
			return ctx.Err()
		},
	}
	returned := make(chan struct{})
	go func() {
		daemon.Run(ctx)
		close(returned)
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("the watcher never started the pending Wi-Fi change")
	}
	select {
	case <-returned:
		t.Fatal("Run returned while a Wi-Fi change was still being applied")
	case <-time.After(100 * time.Millisecond):
	}
	close(release)
	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		t.Fatal("Run never returned after the Wi-Fi change finished")
	}
}
