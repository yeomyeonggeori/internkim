package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/box"
	"github.com/yeomyeonggeori/internkim/internal/companyhost"
	"github.com/yeomyeonggeori/internkim/internal/durablefile"
	"github.com/yeomyeonggeori/internkim/internal/walstream"
)

const takeOverSessionFileName = "take-over-session"

func takeOverSessionPath() string {
	return filepath.Join(companyhost.DefaultBackupDirectoryPath(), takeOverSessionFileName)
}

func keepTheTakeOverSession(sessionPath, accessToken string) error {
	if errorValue := os.MkdirAll(filepath.Dir(sessionPath), 0o700); errorValue != nil {
		return errorValue
	}
	return durablefile.Write(sessionPath, []byte(accessToken+"\n"), 0o600)
}

func startingWithTheTakeOverSession(places box.Places, freshSession func() (string, error)) func(companyID string) error {
	return func(string) error {
		accessToken, errorValue := freshSession()
		if errorValue != nil {
			return errorValue
		}
		return box.KeepTheSession(places, accessToken)
	}
}

func keptForTheTakeOver(claim box.Session, boxStatePath, sessionPath string) (keptOffTheBox, error) {
	companyID := claim.Configuration.Company.ID
	connected, errorValue := box.LoadConnectedTo(boxStatePath, companyID)
	if errorValue != nil {
		return keptOffTheBox{}, fmt.Errorf("opening this box's keys for %s: %w", claim.Configuration.Company.Name, errorValue)
	}
	if !connected.Keyring.HoldsCompanyKey(claim.SealingKey) {
		return keptOffTheBox{}, fmt.Errorf("this box holds no key %s's backups are sealed to, so it cannot take the company over", claim.Configuration.Company.Name)
	}
	destination := companyDestinationOf(claim)
	destination.Store.AccessToken = func() (string, error) { return heldTakeOverSession(sessionPath) }
	return keptOffTheBox{destination: destination, keyring: connected.Keyring, holdsTheCompanyKey: true}, nil
}

func heldTakeOverSession(sessionPath string) (string, error) {
	document, errorValue := os.ReadFile(sessionPath)
	if errorValue != nil {
		return "", errorValue
	}
	return strings.TrimSpace(string(document)), nil
}

func theCompanysLog(kept keptOffTheBox) (companyhost.KeptLog, error) {
	baseBackup, errorValue := walstream.NewestBaseBackup(context.Background(), kept.destination)
	if errorValue != nil {
		return companyhost.KeptLog{}, fmt.Errorf("reading the company's newest base backup to take it over from its log: %w", errorValue)
	}
	return companyhost.KeptLog{
		CompanyID:       kept.destination.CompanyID,
		BaseBackupName:  baseBackup.Name,
		TakenAt:         baseBackup.CreatedAt,
		PackageVersion:  baseBackup.PackageVersion,
		PostgreSQLMajor: baseBackup.PostgreSQLMajor,
	}, nil
}
