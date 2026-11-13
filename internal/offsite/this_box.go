package offsite

import (
	"context"
	"errors"
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
	Manifest       Manifest
	UploadedChunks int
	Pruned         []string
}

func (place ThisBox) Destination() (Destination, box.Keyring, bool, error) {
	connected, errorValue := box.LoadConnected(place.BoxStatePath)
	return place.destinationOf(connected, errorValue)
}

func (place ThisBox) DestinationOf(companyID string) (Destination, box.Keyring, bool, error) {
	connected, errorValue := box.LoadConnectedTo(place.BoxStatePath, companyID)
	return place.destinationOf(connected, errorValue)
}

func (place ThisBox) destinationOf(connected box.Connected, errorValue error) (Destination, box.Keyring, bool, error) {
	if errors.Is(errorValue, box.ErrNoBoxKey) || errors.Is(errorValue, box.ErrNoCompanyInstalled) {
		return Destination{}, box.Keyring{}, false, nil
	}
	if errorValue != nil {
		return Destination{}, box.Keyring{}, false, errorValue
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

func (place ThisBox) Ship(ctx context.Context, keep int, now time.Time, snapshotTo func(*Snapshot) (companyhost.Snapshot, error)) (Shipment, bool, error) {
	destination, keyring, isConnected, errorValue := place.Destination()
	if errorValue != nil || !isConnected {
		return Shipment{}, false, errorValue
	}
	snapshot, errorValue := StartSnapshot(ctx, destination, keyring)
	if errorValue != nil {
		return Shipment{}, true, errorValue
	}
	written, errorValue := snapshotTo(snapshot)
	if errorValue != nil {
		return Shipment{}, true, errorValue
	}
	manifest, errorValue := snapshot.Finish(written.Manifest, now)
	if errorValue != nil {
		return Shipment{}, true, errorValue
	}
	pruned, errorValue := Prune(ctx, destination, keep, now)
	return Shipment{Manifest: manifest, UploadedChunks: snapshot.UploadedChunks(), Pruned: pruned}, true, errorValue
}
