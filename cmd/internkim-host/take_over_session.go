package main

import (
	"os"
	"path/filepath"

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

func asTheBoxTakingOver(place offsite.ThisBox, sessionPath string) offsite.ThisBox {
	place.AccessTokenPath = sessionPath
	return place
}
