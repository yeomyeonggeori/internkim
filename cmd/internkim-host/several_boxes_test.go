package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/box"
	"github.com/yeomyeonggeori/internkim/internal/offsite"
)

const exampleCompanyID = "00000000-0000-4000-8000-000000000001"

type servingBox struct {
	destination offsite.Destination
	errorValue  error
}

func TestAFollowerWaitsForItsBoxToHoldTheCompanyKeyInsteadOfStopping(t *testing.T) {
	place := boxWithAConnectionAndNoCompany(t)
	identity, errorValue := box.LoadOrCreateIdentity(filepath.Join(place.BoxStatePath, "identity.json"))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	served := make(chan servingBox, 1)
	go func() {
		destination, _, errorValue := untilThisBoxServes(ctx, place)
		served <- servingBox{destination: destination, errorValue: errorValue}
	}()

	select {
	case early := <-served:
		t.Fatalf("a box that has installed no company answered %+v instead of waiting", early)
	case <-time.After(200 * time.Millisecond):
	}

	if errorValue := box.MarkInstalledCompany(place.BoxStatePath, exampleCompanyID); errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := box.KeyringFor(identity, place.BoxStatePath, exampleCompanyID, identity.EncryptionPublicKey(), nil); errorValue != nil {
		t.Fatal(errorValue)
	}

	serving := <-served
	if serving.errorValue != nil {
		t.Fatalf("the box holds the company key and the follower stopped with %v", serving.errorValue)
	}
	if serving.destination.CompanyID != exampleCompanyID || serving.destination.SealingKey != identity.EncryptionPublicKey() {
		t.Fatalf("the follower keeps for %s under %s instead of the company the box installed", serving.destination.CompanyID, serving.destination.SealingKey)
	}
}

func boxWithAConnectionAndNoCompany(t *testing.T) offsite.ThisBox {
	t.Helper()
	directory := t.TempDir()
	connectionPath := filepath.Join(directory, "connection.json")
	document := `{"schemaVersion":1,"appURL":"https://company.example.com",` +
		`"company":{"id":"` + exampleCompanyID + `","name":"Example Co","slug":"example"},` +
		`"centralPlane":{"projectURL":"https://project.supabase.co","publishableKey":"publishable"},` +
		`"gatewayURL":"wss://gateway.example.com","agentKey":"` + strings.Repeat("a", 64) + `"}`
	if errorValue := os.WriteFile(connectionPath, []byte(document), 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	return offsite.ThisBox{BoxStatePath: filepath.Join(directory, "box"), ConnectionPath: connectionPath}
}
