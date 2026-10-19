package offsite

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/box"
	"github.com/yeomyeonggeori/internkim/internal/companyhost"
)

type ThisBox struct {
	BoxStatePath    string
	ConnectionPath  string
	AccessTokenPath string
}

type Shipment struct {
	Manifest Manifest
	Pruned   []string
}

func (place ThisBox) Destination() (Destination, box.Keyring, bool, error) {
	connected, errorValue := box.LoadConnected(place.BoxStatePath)
	if errorValue != nil {
		return Destination{}, box.Keyring{}, false, nil
	}
	sealingKey, isHeld := connected.Keyring.SealingKey()
	if !isHeld {
		return Destination{}, box.Keyring{}, false, nil
	}
	connection, errorValue := companyhost.ReadConnection(place.ConnectionPath)
	if errorValue != nil {
		return Destination{}, box.Keyring{}, false, fmt.Errorf("reading the company's connection at %s: %w", place.ConnectionPath, errorValue)
	}
	return Destination{
		Store: Store{
			ProjectURL:     connection.CentralPlane.ProjectURL,
			PublishableKey: connection.CentralPlane.PublishableKey,
			AccessToken:    place.readAccessToken,
		},
		CompanyID:  connected.CompanyID,
		SealingKey: sealingKey,
	}, connected.Keyring, true, nil
}

func (place ThisBox) readAccessToken() (string, error) {
	document, errorValue := os.ReadFile(place.AccessTokenPath)
	if errorValue != nil {
		return "", errorValue
	}
	return strings.TrimSpace(string(document)), nil
}

func (place ThisBox) Ship(ctx context.Context, archivePath string, keep int, now time.Time) (Shipment, bool, error) {
	destination, _, isConnected, errorValue := place.Destination()
	if errorValue != nil || !isConnected {
		return Shipment{}, false, errorValue
	}
	manifest, errorValue := Ship(ctx, archivePath, destination, now)
	if errorValue != nil {
		return Shipment{}, true, errorValue
	}
	pruned, errorValue := Prune(ctx, destination, keep)
	return Shipment{Manifest: manifest, Pruned: pruned}, true, errorValue
}

func (place ThisBox) FetchNewest(ctx context.Context, archivePath string) (Manifest, error) {
	destination, keyring, isConnected, errorValue := place.Destination()
	if errorValue != nil {
		return Manifest{}, errorValue
	}
	if !isConnected {
		return Manifest{}, ErrNoBackup
	}
	manifest, errorValue := Newest(ctx, destination)
	if errorValue != nil {
		return Manifest{}, errorValue
	}
	return manifest, Fetch(ctx, destination, manifest, keyring, archivePath)
}
