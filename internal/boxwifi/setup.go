package boxwifi

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"
)

const (
	defaultOnlineWait             = 45 * time.Second
	defaultSavedNetworkWaitAtBoot = 60 * time.Second
	defaultSetupWindow            = 10 * time.Minute
	connectingWait                = 2 * time.Minute
	onlinePollInterval            = 3 * time.Second
)

var errSetupWindowElapsed = errors.New("the setup window elapsed without a submission")

type submission struct {
	ssid     string
	password string
}

type Setup struct {
	Radio                  Radio
	MadeOn                 time.Time
	HostName               func() (string, error)
	Listen                 func(address string) (net.Listener, error)
	Sleep                  func(context.Context, time.Duration) error
	Now                    func() time.Time
	OnlineWait             time.Duration
	SavedNetworkWaitAtBoot time.Duration
	SetupWindow            time.Duration
	Logf                   func(string, ...any)
	IsOnANetwork           func(ctx context.Context) bool
}

func (setup Setup) Run(ctx context.Context) error {
	if setup.waitUntilOnline(ctx, setup.savedNetworkWaitAtBoot()) {
		return nil
	}
	hasJoinFailed := false
	for {
		if setup.keepsTheSetupNetworkClosed(ctx) {
			return nil
		}
		networks, errorValue := setup.Radio.Scan(ctx)
		if errorValue != nil {
			setup.logf("scanning for nearby networks: %v", errorValue)
			networks = nil
		}
		name := setup.networkName()
		address, errorValue := setup.Radio.OpenSetupNetwork(ctx, name)
		if errorValue != nil {
			setup.closeSetupNetwork(name)
			return fmt.Errorf("opening the %s access point: %w", name, errorValue)
		}
		result, errorValue := setup.serveUntilSubmitted(ctx, address, networks, hasJoinFailed)
		setup.closeSetupNetwork(name)
		if errors.Is(errorValue, errSetupWindowElapsed) {
			if setup.waitUntilOnline(ctx, setup.onlineWait()) {
				return nil
			}
			continue
		}
		if errorValue != nil {
			return errorValue
		}
		if errorValue := setup.Radio.Join(ctx, result.ssid, result.password); errorValue != nil {
			setup.logf("joining %s: %v", result.ssid, errorValue)
			hasJoinFailed = true
			continue
		}
		if setup.waitUntilOnline(ctx, setup.onlineWait()) {
			return nil
		}
		hasJoinFailed = true
	}
}

func (setup Setup) keepsTheSetupNetworkClosed(ctx context.Context) bool {
	if setup.waitWhileConnecting(ctx) && setup.Radio.IsOnline(ctx) {
		return true
	}
	if setup.IsOnANetwork == nil || !setup.IsOnANetwork(ctx) {
		return false
	}
	setup.logf("joined a network that does not reach the internet, so the setup network stays closed")
	return true
}

func (setup Setup) waitWhileConnecting(ctx context.Context) bool {
	if !setup.Radio.IsConnecting(ctx) {
		return false
	}
	setup.logf("NetworkManager is connecting to a network, so the setup network waits for it")
	waitUntil(ctx, connectingWait, setup.now, setup.sleep, func() bool { return !setup.Radio.IsConnecting(ctx) })
	return true
}

func (setup Setup) serveUntilSubmitted(ctx context.Context, address string, networks []Network, hasJoinFailed bool) (submission, error) {
	listener, errorValue := setup.listen(fmt.Sprintf("%s:80", address))
	if errorValue != nil {
		return submission{}, fmt.Errorf("listening for the setup page on %s: %w", address, errorValue)
	}
	submitted := make(chan submission, 1)
	server := &http.Server{Handler: newPage(networks, hasJoinFailed, submitted)}
	serveErrors := make(chan error, 1)
	go func() { serveErrors <- server.Serve(listener) }()
	defer func() {
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownContext)
	}()
	window := time.NewTimer(setup.setupWindow())
	defer window.Stop()
	select {
	case <-ctx.Done():
		return submission{}, ctx.Err()
	case <-window.C:
		return submission{}, errSetupWindowElapsed
	case result := <-submitted:
		return result, nil
	}
}

func (setup Setup) waitUntilOnline(ctx context.Context, limit time.Duration) bool {
	return waitUntil(ctx, limit, setup.now, setup.sleep, func() bool { return setup.Radio.IsOnline(ctx) })
}

func (setup Setup) now() time.Time {
	if setup.Now != nil {
		return setup.Now()
	}
	return time.Now()
}

func (setup Setup) closeSetupNetwork(name string) {
	if errorValue := setup.Radio.CloseSetupNetwork(context.Background()); errorValue != nil {
		setup.logf("closing the %s access point: %v", name, errorValue)
	}
}

func (setup Setup) listen(address string) (net.Listener, error) {
	if setup.Listen != nil {
		return setup.Listen(address)
	}
	return net.Listen("tcp", address)
}

func (setup Setup) sleep(ctx context.Context, wait time.Duration) error {
	if setup.Sleep != nil {
		return setup.Sleep(ctx, wait)
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

func (setup Setup) onlineWait() time.Duration {
	if setup.OnlineWait > 0 {
		return setup.OnlineWait
	}
	return defaultOnlineWait
}

func (setup Setup) savedNetworkWaitAtBoot() time.Duration {
	if setup.SavedNetworkWaitAtBoot > 0 {
		return setup.SavedNetworkWaitAtBoot
	}
	return defaultSavedNetworkWaitAtBoot
}

func (setup Setup) setupWindow() time.Duration {
	if setup.SetupWindow > 0 {
		return setup.SetupWindow
	}
	return defaultSetupWindow
}

func (setup Setup) networkName() string {
	hostName, errorValue := setup.hostName()
	if errorValue != nil {
		setup.logf("naming the setup network without this computer's host name: %v", errorValue)
	}
	return SetupNetworkNameFor(hostName, setup.MadeOn)
}

func (setup Setup) hostName() (string, error) {
	if setup.HostName != nil {
		return setup.HostName()
	}
	return os.Hostname()
}

func (setup Setup) logf(format string, arguments ...any) {
	if setup.Logf != nil {
		setup.Logf(format, arguments...)
		return
	}
	log.Printf(format, arguments...)
}
