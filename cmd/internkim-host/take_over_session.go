package main

import (
	"os"
	"path/filepath"

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

func startingWithTheTakeOverSession(places box.Places, freshSession func() (string, error)) func(companyID string) error {
	return func(string) error {
		accessToken, errorValue := freshSession()
		if errorValue != nil {
			return errorValue
		}
		return box.KeepTheSession(places, accessToken)
	}
}

func asTheBoxTakingOver(place offsite.ThisBox, sessionPath string) offsite.ThisBox {
	place.AccessTokenPath = sessionPath
	return place
}
