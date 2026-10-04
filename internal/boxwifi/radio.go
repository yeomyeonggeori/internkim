package boxwifi

import (
	"context"
	"time"
)

const (
	SetupNetworkName       = "kimmini"
	SetupNetworkPassword   = "intern-kim"
	setupNetworkDateLayout = "060102"
)

func SetupNetworkNameFor(madeOn time.Time) string {
	if madeOn.IsZero() {
		return SetupNetworkName
	}
	return SetupNetworkName + "-" + madeOn.Format(setupNetworkDateLayout)
}

type Network struct {
	SSID          string
	SignalPercent int
	IsSecured     bool
	IsConnected   bool
}

type Radio interface {
	IsOnline(ctx context.Context) bool
	Scan(ctx context.Context) ([]Network, error)
	OpenSetupNetwork(ctx context.Context, name string) (address string, errorValue error)
	CloseSetupNetwork(ctx context.Context) error
	Join(ctx context.Context, ssid, password string) error
}
