package boxwifi

import (
	"context"
	"time"
)

const (
	SetupNetworkName       = "kimmini"
	SetupNetworkPassword   = "intern-kim"
	boxNameSuffixLength    = 4
	setupNetworkDateLayout = "060102"
)

func SetupNetworkNameFor(boxPublicKey string, madeOn time.Time) string {
	name := SetupNetworkName
	if suffix := boxNameSuffix(boxPublicKey); suffix != "" {
		name += "-" + suffix
	}
	if !madeOn.IsZero() {
		name += "-" + madeOn.Format(setupNetworkDateLayout)
	}
	return name
}

func boxNameSuffix(boxPublicKey string) string {
	if len(boxPublicKey) < boxNameSuffixLength {
		return ""
	}
	return boxPublicKey[len(boxPublicKey)-boxNameSuffixLength:]
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
