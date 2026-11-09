package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/yeomyeonggeori/internkim/internal/companyhost"
	"github.com/yeomyeonggeori/internkim/internal/filestream"
	"github.com/yeomyeonggeori/internkim/internal/offsite"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

const fileJournalSocketMode = 0o666

func runFileFollow(arguments []string) error {
	if len(arguments) != 0 {
		return fmt.Errorf("file-follow takes no arguments")
	}
	if errorValue := companyhost.RequireAdministrator(); errorValue != nil {
		return errorValue
	}
	destination, keyring, isConnected, errorValue := thisBox().Destination()
	if errorValue != nil {
		return errorValue
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()
	if !isConnected {
		isABox, errorValue := thisComputerIsABox()
		if errorValue != nil {
			return errorValue
		}
		if isABox {
			return errNotAServingBox
		}
		return filestream.NewUnjournaledBarrier().Serve(ctx, blueclaw.CompanyHostFileJournalSocketPath, fileJournalSocketMode)
	}
	outer := ctx
	hasSeveralBoxes, errorValue := untilTheBoxCountIsKnown(ctx)
	if errorValue != nil {
		return errorValue
	}
	watched, cancel := whileSeveralBoxesIs(ctx, hasSeveralBoxes)
	defer cancel()
	if !hasSeveralBoxes {
		return endedByTheBoxCount(filestream.NewUnjournaledBarrier().Serve(watched, blueclaw.CompanyHostFileJournalSocketPath, fileJournalSocketMode), outer)
	}
	ctx = watched
	barrier := filestream.NewBarrier(blueclaw.CompanyHostStateRoot, companyhost.BarrierRole)
	if errorValue := barrier.Prepare(); errorValue != nil {
		return errorValue
	}
	serving := make(chan error, 1)
	go func() {
		serving <- barrier.Serve(ctx, blueclaw.CompanyHostFileJournalSocketPath, fileJournalSocketMode)
	}()
	following := make(chan error, 1)
	go func() {
		following <- filestream.Follower{
			Roots:                companyhost.JournaledFileRoots(blueclaw.LinuxCompanyHostLayout(), filestream.BarrierDirectoryName),
			Journal:              filestream.Journal{Destination: destination},
			Keyring:              keyring,
			ScratchDirectoryPath: blueclaw.CompanyHostBackupsPath,
			Barrier:              barrier,
		}.Run(ctx)
	}()
	select {
	case errorValue := <-serving:
		return endedByTheBoxCount(errorValue, outer)
	case errorValue := <-following:
		return endedByTheBoxCount(errorValue, outer)
	}
}

func endedByTheBoxCount(errorValue error, outer context.Context) error {
	if outer.Err() == nil && errors.Is(errorValue, context.Canceled) {
		return nil
	}
	return errorValue
}

func replayTheFileJournal(apply companyhost.FileJournalApplier) (int, error) {
	destination, keyring, isConnected, errorValue := thisBox().Destination()
	if errorValue != nil {
		return 0, errorValue
	}
	if !isConnected {
		return 0, errNotAServingBox
	}
	cache := offsite.ChunkCache{DirectoryPath: filepath.Join(blueclaw.CompanyHostBackupsPath, "file-chunks")}
	replayed, errorValue := filestream.Journal{Destination: destination}.Replay(context.Background(), keyring, cache, func(entry filestream.Entry, contents io.Reader) error {
		return apply(entry.IsFull, entry.Removed, contents)
	})
	if errors.Is(errorValue, filestream.ErrNoFullEntry) {
		return 0, companyhost.ErrNoFileJournal
	}
	return replayed, errorValue
}
