package boxwifi

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	setupConnectionName        = "internkim-setup"
	setupNetworkAddress        = "10.42.0.1"
	defaultCaptiveDNSPath      = "/etc/NetworkManager/dnsmasq-shared.d/internkim-setup.conf"
	joinAttempts               = 4
	joinKeyfilePrefix          = "internkim-"
	defaultRescanMode          = "auto"
	joinAttemptInterval        = 3 * time.Second
	wpa3AccessPointWaitSeconds = "15"
)

type NetworkManagerRadio struct {
	Run              func(ctx context.Context, name string, arguments ...string) ([]byte, error)
	CaptiveDNSPath   string
	JoinRetryWait    time.Duration
	KeyfileDirectory string
	ReachesPlane     func(ctx context.Context) bool
	Now              func() time.Time
	Sleep            func(ctx context.Context, wait time.Duration) error
	RescanMode       string
}

func (radio NetworkManagerRadio) run(ctx context.Context, arguments ...string) ([]byte, error) {
	if radio.Run != nil {
		return radio.Run(ctx, "nmcli", arguments...)
	}
	command := exec.CommandContext(ctx, "nmcli", arguments...)
	command.Env = append(os.Environ(), "LC_ALL=C.UTF-8")
	return command.CombinedOutput()
}

func (radio NetworkManagerRadio) IsOnline(ctx context.Context) bool {
	output, errorValue := radio.run(ctx, "-t", "-f", "CONNECTIVITY", "general", "status")
	if errorValue == nil && strings.TrimSpace(string(output)) == "full" {
		return true
	}
	return radio.ReachesPlane != nil && radio.ReachesPlane(ctx)
}

func (radio NetworkManagerRadio) rescanMode() string {
	if radio.RescanMode != "" {
		return radio.RescanMode
	}
	return defaultRescanMode
}

func (radio NetworkManagerRadio) Scan(ctx context.Context) ([]Network, error) {
	output, errorValue := radio.run(ctx, "-t", "-f", "SSID,SIGNAL,SECURITY,IN-USE", "device", "wifi", "list", "--rescan", radio.rescanMode())
	if errorValue != nil {
		return nil, fmt.Errorf("scanning for nearby networks: %w", errorValue)
	}
	return parseScanOutput(output), nil
}

func (radio NetworkManagerRadio) OpenSetupNetwork(ctx context.Context, name string) (string, error) {
	radio.run(ctx, "connection", "delete", "id", setupConnectionName)
	if errorValue := radio.answerEveryNameWithTheBox(); errorValue != nil {
		return "", errorValue
	}
	if _, errorValue := radio.run(ctx, "connection", "add", "type", "wifi", "ifname", "*", "con-name", setupConnectionName,
		"autoconnect", "no", "ssid", name,
		"802-11-wireless.mode", "ap", "ipv4.method", "shared", "ipv4.addresses", setupNetworkAddress+"/24",
		"wifi-sec.key-mgmt", "sae", "wifi-sec.psk", SetupNetworkPassword); errorValue != nil {
		return "", fmt.Errorf("creating the %s access point: %w", SetupNetworkName, errorValue)
	}
	if _, errorValue := radio.run(ctx, "--wait", wpa3AccessPointWaitSeconds, "connection", "up", "id", setupConnectionName); errorValue != nil {
		if _, modifyError := radio.run(ctx, "connection", "modify", "id", setupConnectionName, "wifi-sec.key-mgmt", "wpa-psk"); modifyError != nil {
			return "", fmt.Errorf("falling back to WPA2 for the %s access point: %w", SetupNetworkName, modifyError)
		}
		if _, errorValue := radio.run(ctx, "connection", "up", "id", setupConnectionName); errorValue != nil {
			return "", fmt.Errorf("starting the %s access point: %w", SetupNetworkName, errorValue)
		}
	}
	return radio.setupNetworkAddress(ctx)
}

func (radio NetworkManagerRadio) setupNetworkAddress(ctx context.Context) (string, error) {
	output, errorValue := radio.run(ctx, "-g", "IP4.ADDRESS", "connection", "show", "id", setupConnectionName)
	if errorValue != nil {
		return "", fmt.Errorf("reading the %s access point's address: %w", SetupNetworkName, errorValue)
	}
	address := strings.TrimSpace(string(output))
	if slash := strings.Index(address, "/"); slash >= 0 {
		address = address[:slash]
	}
	return address, nil
}

func (radio NetworkManagerRadio) CloseSetupNetwork(ctx context.Context) error {
	var failures []error
	if _, errorValue := radio.run(ctx, "connection", "down", "id", setupConnectionName); errorValue != nil {
		failures = append(failures, fmt.Errorf("closing the %s access point: %w", SetupNetworkName, errorValue))
	}
	if _, errorValue := radio.run(ctx, "connection", "delete", "id", setupConnectionName); errorValue != nil {
		failures = append(failures, fmt.Errorf("deleting the %s access point profile: %w", SetupNetworkName, errorValue))
	}
	if errorValue := os.Remove(radio.captiveDNSPath()); errorValue != nil && !os.IsNotExist(errorValue) {
		failures = append(failures, fmt.Errorf("removing %s: %w", radio.captiveDNSPath(), errorValue))
	}
	return errors.Join(failures...)
}

func (radio NetworkManagerRadio) answerEveryNameWithTheBox() error {
	path := radio.captiveDNSPath()
	if errorValue := os.MkdirAll(filepath.Dir(path), 0o755); errorValue != nil {
		return fmt.Errorf("preparing %s: %w", filepath.Dir(path), errorValue)
	}
	if errorValue := os.WriteFile(path, []byte("address=/#/"+setupNetworkAddress+"\n"), 0o644); errorValue != nil {
		return fmt.Errorf("writing %s: %w", path, errorValue)
	}
	return nil
}

func (radio NetworkManagerRadio) captiveDNSPath() string {
	if radio.CaptiveDNSPath != "" {
		return radio.CaptiveDNSPath
	}
	return defaultCaptiveDNSPath
}

func (radio NetworkManagerRadio) Join(ctx context.Context, ssid, password string) error {
	radio.run(ctx, "connection", "delete", "id", ssid)
	if output, errorValue := radio.loadProfile(ctx, joinKeyfilePrefix+ssid, ssid, ssid, password); errorValue != nil {
		return fmt.Errorf("saving %s: %s: %w", ssid, strings.TrimSpace(string(output)), errorValue)
	}
	var output []byte
	var errorValue error
	for attempt := 1; attempt <= joinAttempts; attempt++ {
		radio.run(ctx, "device", "wifi", "rescan", "ssid", ssid)
		output, errorValue = radio.run(ctx, "--wait", "30", "connection", "up", "id", ssid)
		if errorValue == nil {
			return nil
		}
		if !isNotYetSeen(output) || attempt == joinAttempts {
			break
		}
		if waitError := waitFor(ctx, radio.joinRetryWait()); waitError != nil {
			return waitError
		}
	}
	return fmt.Errorf("joining %s: %s: %w", ssid, strings.TrimSpace(string(output)), errorValue)
}

func isNotYetSeen(output []byte) bool {
	text := strings.ToLower(string(output))
	return strings.Contains(text, "no network with ssid") || strings.Contains(text, "no suitable device")
}

func (radio NetworkManagerRadio) joinRetryWait() time.Duration {
	if radio.JoinRetryWait > 0 {
		return radio.JoinRetryWait
	}
	return joinAttemptInterval
}

func waitFor(ctx context.Context, wait time.Duration) error {
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func parseScanOutput(output []byte) []Network {
	var networks []Network
	for _, line := range strings.Split(strings.TrimRight(string(output), "\n"), "\n") {
		if line == "" {
			continue
		}
		fields := splitTerseFields(line)
		if len(fields) < 3 || fields[0] == "" {
			continue
		}
		signalPercent, _ := strconv.Atoi(fields[1])
		networks = append(networks, Network{
			SSID:          fields[0],
			SignalPercent: signalPercent,
			IsSecured:     fields[2] != "" && fields[2] != "--",
			IsConnected:   len(fields) > 3 && strings.TrimSpace(fields[3]) == "*",
		})
	}
	sort.SliceStable(networks, func(i, j int) bool {
		return networks[i].SignalPercent > networks[j].SignalPercent
	})
	return dedupeBySSID(networks)
}

func dedupeBySSID(networks []Network) []Network {
	positions := make(map[string]int, len(networks))
	deduped := make([]Network, 0, len(networks))
	for _, network := range networks {
		position, isSeen := positions[network.SSID]
		if isSeen {
			deduped[position].IsConnected = deduped[position].IsConnected || network.IsConnected
			continue
		}
		positions[network.SSID] = len(deduped)
		deduped = append(deduped, network)
	}
	return deduped
}

func splitTerseFields(line string) []string {
	var fields []string
	var current strings.Builder
	escaped := false
	for _, character := range line {
		switch {
		case escaped:
			current.WriteRune(character)
			escaped = false
		case character == '\\':
			escaped = true
		case character == ':':
			fields = append(fields, current.String())
			current.Reset()
		default:
			current.WriteRune(character)
		}
	}
	fields = append(fields, current.String())
	return fields
}
