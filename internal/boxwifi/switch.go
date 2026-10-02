package boxwifi

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	pendingConnectionName = "internkim-pending"
	switchJoinWaitSeconds = "30"
	switchOnlineWait      = 45 * time.Second
	switchSettleTimeout   = 45 * time.Second
)

func (radio NetworkManagerRadio) Switch(ctx context.Context, ssid, password string) error {
	previous, errorValue := radio.activeWifiConnectionName(ctx)
	if errorValue != nil {
		return fmt.Errorf("reading the active Wi-Fi connection before switching to %s: %w", ssid, errorValue)
	}
	radio.run(ctx, "connection", "delete", "id", pendingConnectionName)
	pendingFileName := pendingConnectionName + "-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	if output, errorValue := radio.loadProfile(ctx, pendingFileName, pendingConnectionName, ssid, password); errorValue != nil {
		radio.discardPending(ctx, pendingFileName)
		return fmt.Errorf("creating a connection profile for %s: %s: %w", ssid, strings.TrimSpace(string(output)), errorValue)
	}
	radio.run(ctx, "device", "wifi", "rescan", "ssid", ssid)
	if output, errorValue := radio.run(ctx, "--wait", switchJoinWaitSeconds, "connection", "up", "id", pendingConnectionName); errorValue != nil {
		radio.revertTo(ctx, previous, pendingFileName)
		return fmt.Errorf("joining %s: %s: %w", ssid, strings.TrimSpace(string(output)), errorValue)
	}
	if !radio.waitUntilOnline(ctx, switchOnlineWait) {
		radio.revertTo(ctx, previous, pendingFileName)
		return fmt.Errorf("switching to %s: the box never came back online", ssid)
	}
	return radio.keepAs(ctx, ssid)
}

func (radio NetworkManagerRadio) activeWifiConnectionName(ctx context.Context) (string, error) {
	output, errorValue := radio.run(ctx, "-t", "-f", "NAME,TYPE", "connection", "show", "--active")
	if errorValue != nil {
		return "", errorValue
	}
	for _, line := range strings.Split(strings.TrimRight(string(output), "\n"), "\n") {
		if line == "" {
			continue
		}
		fields := splitTerseFields(line)
		if len(fields) >= 2 && fields[1] == "802-11-wireless" {
			return fields[0], nil
		}
	}
	return "", nil
}

func (radio NetworkManagerRadio) keepAs(ctx context.Context, ssid string) error {
	ctx, cancel := settleContext(ctx)
	defer cancel()
	radio.run(ctx, "connection", "delete", "id", ssid)
	if output, errorValue := radio.run(ctx, "connection", "modify", "id", pendingConnectionName, "connection.id", ssid); errorValue != nil {
		return fmt.Errorf("naming the new connection %s: %s: %w", ssid, strings.TrimSpace(string(output)), errorValue)
	}
	return nil
}

func (radio NetworkManagerRadio) discardPending(ctx context.Context, pendingFileName string) {
	radio.run(ctx, "connection", "delete", "id", pendingConnectionName)
	radio.removeKeyfile(pendingFileName)
}

func (radio NetworkManagerRadio) revertTo(ctx context.Context, previous, pendingFileName string) {
	ctx, cancel := settleContext(ctx)
	defer cancel()
	radio.discardPending(ctx, pendingFileName)
	if previous != "" {
		radio.run(ctx, "connection", "up", "id", previous)
	}
}

func settleContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), switchSettleTimeout)
}

func (radio NetworkManagerRadio) waitUntilOnline(ctx context.Context, limit time.Duration) bool {
	return waitUntil(ctx, limit, radio.now, radio.sleep, func() bool { return radio.isReady(ctx) })
}

func (radio NetworkManagerRadio) isReady(ctx context.Context) bool {
	if radio.ReachesPlane != nil {
		return radio.ReachesPlane(ctx)
	}
	return radio.IsOnline(ctx)
}

func (radio NetworkManagerRadio) now() time.Time {
	if radio.Now != nil {
		return radio.Now()
	}
	return time.Now()
}

func (radio NetworkManagerRadio) sleep(ctx context.Context, wait time.Duration) error {
	if radio.Sleep != nil {
		return radio.Sleep(ctx, wait)
	}
	return waitFor(ctx, wait)
}
