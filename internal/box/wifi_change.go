package box

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"time"
)

const (
	wifiChangeCheckInterval = time.Minute
	maximumNearbyNetworks   = 50
	maximumSSIDBytes        = 32
)

type NearbyNetwork struct {
	SSID          string `json:"ssid"`
	SignalPercent int    `json:"signalPercent"`
	IsSecured     bool   `json:"isSecured"`
	IsConnected   bool   `json:"isConnected"`
}

func reportableNetworks(networks []NearbyNetwork) []NearbyNetwork {
	candidates := make([]NearbyNetwork, 0, len(networks))
	for _, network := range networks {
		if len(network.SSID) == 0 || len(network.SSID) > maximumSSIDBytes {
			continue
		}
		candidates = append(candidates, network)
	}
	sort.SliceStable(candidates, func(first, second int) bool {
		return candidates[first].SignalPercent > candidates[second].SignalPercent
	})
	seen := make(map[string]bool, len(candidates))
	reportable := make([]NearbyNetwork, 0, len(candidates))
	for _, network := range candidates {
		if seen[network.SSID] {
			continue
		}
		seen[network.SSID] = true
		reportable = append(reportable, network)
	}
	if len(reportable) > maximumNearbyNetworks {
		reportable = reportable[:maximumNearbyNetworks]
	}
	return reportable
}

type PendingWifiChange struct {
	CompanyID string       `json:"-"`
	RequestID string       `json:"requestID"`
	Sealed    SealedSecret `json:"sealed"`
}

type WifiChangeOutcome string

const (
	WifiChangeJoined WifiChangeOutcome = "joined"
	WifiChangeFailed WifiChangeOutcome = "failed"
)

func (client Client) PendingWifiChange(ctx context.Context, identity Identity, nearbyNetworks []NearbyNetwork) (*PendingWifiChange, error) {
	var body []byte
	if nearbyNetworks != nil {
		encoded, errorValue := json.Marshal(map[string][]NearbyNetwork{"nearbyNetworks": reportableNetworks(nearbyNetworks)})
		if errorValue != nil {
			return nil, errorValue
		}
		body = encoded
	}
	response, errorValue := client.post(ctx, identity, "/api/box/wifi", body)
	if errorValue != nil {
		return nil, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if response.StatusCode != http.StatusOK {
		return nil, refusalOf(response, "asking for this box's pending Wi-Fi change")
	}
	var answered struct {
		CompanyID string             `json:"companyID"`
		Change    *PendingWifiChange `json:"change"`
	}
	if errorValue := json.NewDecoder(response.Body).Decode(&answered); errorValue != nil {
		return nil, fmt.Errorf("asking for this box's pending Wi-Fi change: %w", errorValue)
	}
	if answered.Change == nil {
		return nil, nil
	}
	answered.Change.CompanyID = answered.CompanyID
	return answered.Change, nil
}

func (client Client) ReportWifiChange(ctx context.Context, identity Identity, requestID string, result WifiChangeOutcome) error {
	body, errorValue := json.Marshal(map[string]string{"requestID": requestID, "result": string(result)})
	if errorValue != nil {
		return errorValue
	}
	response, errorValue := client.post(ctx, identity, "/api/box/wifi/outcome", body)
	if errorValue != nil {
		return errorValue
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return refusalOf(response, "reporting this box's Wi-Fi change outcome")
	}
	return nil
}

func (daemon Daemon) watchForWifiChanges(ctx context.Context, identity Identity) {
	var handledRequestID string
	var handledResult WifiChangeOutcome
	for {
		if errorValue := daemon.wifiChangeSleep(ctx, wifiChangeCheckInterval); errorValue != nil {
			return
		}
		handledRequestID, handledResult = daemon.checkForWifiChange(ctx, identity, handledRequestID, handledResult)
	}
}

func (daemon Daemon) checkForWifiChange(ctx context.Context, identity Identity, handledRequestID string, handledResult WifiChangeOutcome) (string, WifiChangeOutcome) {
	change, errorValue := daemon.Client.PendingWifiChange(ctx, identity, daemon.scanNearbyNetworks(ctx))
	if errorValue != nil {
		log.Printf("asking for a Wi-Fi change: %v", errorValue)
		return handledRequestID, handledResult
	}
	if change == nil {
		return handledRequestID, handledResult
	}
	if change.RequestID == handledRequestID {
		daemon.reportWifiChange(ctx, identity, handledRequestID, handledResult)
		return handledRequestID, handledResult
	}
	result := daemon.changeWifiFor(ctx, identity, *change)
	daemon.reportWifiChange(ctx, identity, change.RequestID, result)
	return change.RequestID, result
}

func (daemon Daemon) scanNearbyNetworks(ctx context.Context) []NearbyNetwork {
	if daemon.ScanWifi == nil {
		return nil
	}
	networks, errorValue := daemon.ScanWifi(ctx)
	if errorValue != nil {
		log.Printf("scanning for nearby Wi-Fi networks: %v", errorValue)
		return nil
	}
	if networks == nil {
		return []NearbyNetwork{}
	}
	return networks
}

func (daemon Daemon) changeWifiFor(ctx context.Context, identity Identity, change PendingWifiChange) WifiChangeOutcome {
	purpose := WifiNetworkPurpose(change.CompanyID, identity.EncryptionPublicKey(), change.RequestID)
	opened, errorValue := identity.OpenSecret(change.Sealed, purpose)
	if errorValue != nil {
		log.Printf("opening the Wi-Fi change %s: %v", change.RequestID, errorValue)
		return WifiChangeFailed
	}
	var network struct {
		SSID     string `json:"ssid"`
		Password string `json:"password"`
	}
	if errorValue := json.Unmarshal([]byte(opened), &network); errorValue != nil {
		log.Printf("the Wi-Fi change %s: %v", change.RequestID, errorValue)
		return WifiChangeFailed
	}
	log.Printf("switching to the Wi-Fi network %s", network.SSID)
	if errorValue := daemon.ChangeWifi(ctx, network.SSID, network.Password); errorValue != nil {
		log.Printf("switching to %s: %v", network.SSID, errorValue)
		return WifiChangeFailed
	}
	return WifiChangeJoined
}

func (daemon Daemon) reportWifiChange(ctx context.Context, identity Identity, requestID string, result WifiChangeOutcome) {
	if errorValue := daemon.Client.ReportWifiChange(ctx, identity, requestID, result); errorValue != nil {
		log.Printf("reporting the Wi-Fi change %s as %s: %v", requestID, result, errorValue)
	}
}

func (daemon Daemon) wifiChangeSleep(ctx context.Context, wait time.Duration) error {
	if daemon.WifiChangeSleep != nil {
		return daemon.WifiChangeSleep(ctx, wait)
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
