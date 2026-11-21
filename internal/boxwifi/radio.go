package boxwifi

import (
	"context"
	"time"
	"unicode/utf8"
)

const (
	EmptyBoxHostName       = "kimmini"
	SetupNetworkPassword   = "intern-kim"
	setupNetworkDateLayout = "060102"
	setupNetworkNameLimit  = 32
)

func SetupNetworkNameFor(hostName string, madeOn time.Time) string {
	if hostName == "" {
		hostName = EmptyBoxHostName
	}
	if madeOn.IsZero() {
		return withinSetupNetworkNameLimit(hostName, "")
	}
	return withinSetupNetworkNameLimit(hostName, "-"+madeOn.Format(setupNetworkDateLayout))
}

func withinSetupNetworkNameLimit(hostName, suffix string) string {
	room := setupNetworkNameLimit - len(suffix)
	for len(hostName) > room {
		_, lastRuneSize := utf8.DecodeLastRuneInString(hostName)
		hostName = hostName[:len(hostName)-lastRuneSize]
	}
	return hostName + suffix
}

type Network struct {
	SSID          string
	SignalPercent int
	IsSecured     bool
	IsConnected   bool
}

type Radio interface {
	IsOnline(ctx context.Context) bool
	IsConnecting(ctx context.Context) bool
	Scan(ctx context.Context) ([]Network, error)
	OpenSetupNetwork(ctx context.Context, name string) (address string, errorValue error)
	CloseSetupNetwork(ctx context.Context) error
	Join(ctx context.Context, ssid, password string) error
}
