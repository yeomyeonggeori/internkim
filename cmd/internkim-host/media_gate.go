package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/yeomyeonggeori/internkim/internal/filestream"
	"github.com/yeomyeonggeori/internkim/internal/mediagate"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

func runMediaGate(arguments []string) error {
	if len(arguments) != 0 {
		return fmt.Errorf("media-gate takes no arguments")
	}
	upstream, errorValue := url.Parse("http://" + blueclaw.BuzzMediaServerAddress)
	if errorValue != nil {
		return errorValue
	}
	gate := mediagate.Gate{Upstream: upstream}
	isABox, errorValue := thisComputerIsABox()
	if errorValue != nil {
		return errorValue
	}
	if isABox {
		gate.KeepFiles = func(ctx context.Context) error {
			return filestream.AskToKeep(ctx, blueclaw.CompanyHostFileJournalSocketPath)
		}
	}
	return http.ListenAndServe(blueclaw.BuzzMediaAddress, gate.Handler())
}

func thisComputerIsABox() (bool, error) {
	_, errorValue := os.Stat(filepath.Join(blueclaw.CompanyHostBoxStatePath, "identity.json"))
	if errors.Is(errorValue, fs.ErrNotExist) {
		return false, nil
	}
	return errorValue == nil, errorValue
}
