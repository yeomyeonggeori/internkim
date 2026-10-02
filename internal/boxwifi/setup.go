package boxwifi

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"time"
)

const (
	defaultOnlineWait             = 45 * time.Second
	defaultSavedNetworkWaitAtBoot = 60 * time.Second
	defaultSetupWindow            = 10 * time.Minute
	onlinePollInterval            = 3 * time.Second
)

var errSetupWindowElapsed = errors.New("the setup window elapsed without a submission")

type submission struct {
	ssid     string
	password string
}

type Setup struct {
	Radio                  Radio
	NetworkName            string
	Listen                 func(address string) (net.Listener, error)
	Sleep                  func(context.Context, time.Duration) error
	Now                    func() time.Time
	OnlineWait             time.Duration
	SavedNetworkWaitAtBoot time.Duration
	SetupWindow            time.Duration
	Logf                   func(string, ...any)
}

func (setup Setup) Run(ctx context.Context) error {
	if setup.waitUntilOnline(ctx, setup.savedNetworkWaitAtBoot()) {
		return nil
	}
	hasJoinFailed := false
	for {
		networks, errorValue := setup.Radio.Scan(ctx)
		if errorValue != nil {
			setup.logf("scanning for nearby networks: %v", errorValue)
			networks = nil
		}
		address, errorValue := setup.Radio.OpenSetupNetwork(ctx, setup.networkName())
		if errorValue != nil {
			setup.closeSetupNetwork()
			return fmt.Errorf("opening the %s access point: %w", setup.networkName(), errorValue)
		}
		result, errorValue := setup.serveUntilSubmitted(ctx, address, networks, hasJoinFailed)
		setup.closeSetupNetwork()
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

func (setup Setup) closeSetupNetwork() {
	if errorValue := setup.Radio.CloseSetupNetwork(context.Background()); errorValue != nil {
		setup.logf("closing the %s access point: %v", setup.networkName(), errorValue)
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
	if setup.NetworkName != "" {
		return setup.NetworkName
	}
	return SetupNetworkName
}

func (setup Setup) logf(format string, arguments ...any) {
	if setup.Logf != nil {
		setup.Logf(format, arguments...)
		return
	}
	log.Printf(format, arguments...)
}
