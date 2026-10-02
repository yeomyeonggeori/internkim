package boxwifi

import "context"

const (
	SetupNetworkName     = "kimmini"
	SetupNetworkPassword = "intern-kim"
	boxNameSuffixLength  = 4
)

func SetupNetworkNameFor(boxPublicKey string) string {
	if len(boxPublicKey) < boxNameSuffixLength {
		return SetupNetworkName
	}
	return SetupNetworkName + "-" + boxPublicKey[len(boxPublicKey)-boxNameSuffixLength:]
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
