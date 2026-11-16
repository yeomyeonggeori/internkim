package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/yeomyeonggeori/internkim/internal/box"
	"github.com/yeomyeonggeori/internkim/internal/companyhost"
	"github.com/yeomyeonggeori/internkim/internal/durablefile"
	"github.com/yeomyeonggeori/internkim/internal/offsite"
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

func startingWithTheTakeOverSession(places box.Places, sessionPath string, before func(companyID string) error) func(companyID string) error {
	return func(companyID string) error {
		if errorValue := before(companyID); errorValue != nil {
			return errorValue
		}
		session, errorValue := os.ReadFile(sessionPath)
		if errorValue != nil {
			return fmt.Errorf("reading the session this box handed the take-over at %s: %w", sessionPath, errorValue)
		}
		return box.KeepTheSession(places, strings.TrimSpace(string(session)))
	}
}

func asTheBoxTakingOver(place offsite.ThisBox, sessionPath string) offsite.ThisBox {
	place.AccessTokenPath = sessionPath
	return place
}
