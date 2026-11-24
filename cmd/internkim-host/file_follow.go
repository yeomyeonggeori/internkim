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
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, os.Interrupt)
	defer stop()
	return keepWhatThisBoxKeeps(ctx, func(_, watched context.Context, kept keptOffTheBox) error {
		return serveTheFileJournal(watched, kept, blueclaw.CompanyHostFileJournalSocketPath)
	})
}

func serveTheFileJournal(ctx context.Context, kept keptOffTheBox, socketPath string) error {
	if !kept.holdsTheCompanyKey {
		return filestream.NewUnjournaledBarrier().Serve(ctx, socketPath, fileJournalSocketMode)
	}
	barrier := filestream.NewBarrier(blueclaw.CompanyHostStateRoot, companyhost.BarrierRole)
	if errorValue := barrier.Prepare(); errorValue != nil {
		return errorValue
	}
	serving := make(chan error, 1)
	go func() {
		serving <- barrier.Serve(ctx, socketPath, fileJournalSocketMode)
	}()
	following := make(chan error, 1)
	go func() {
		following <- filestream.Follower{
			Roots:                companyhost.JournaledFileRoots(blueclaw.LinuxCompanyHostLayout(), filestream.BarrierDirectoryName),
			Journal:              filestream.Journal{Destination: kept.destination},
			Keyring:              kept.keyring,
			ScratchDirectoryPath: blueclaw.CompanyHostBackupsPath,
			Barrier:              barrier,
		}.Run(ctx)
	}()
	select {
	case errorValue := <-serving:
		return errorValue
	case errorValue := <-following:
		return errorValue
	}
}

func replayTheFileJournal(kept keptOffTheBox, through *int64) companyhost.FileJournalReplay {
	return func(apply companyhost.FileJournalApplier) (int, error) {
		cache := offsite.ChunkCache{DirectoryPath: filepath.Join(blueclaw.CompanyHostBackupsPath, "file-chunks")}
		replayed, errorValue := filestream.Journal{Destination: kept.destination}.ReplayThrough(context.Background(), kept.keyring, cache, through, func(entry filestream.Entry, contents io.Reader) error {
			return apply(entry.IsFull, entry.Removed, contents)
		})
		if errors.Is(errorValue, filestream.ErrNoFullEntry) {
			return 0, companyhost.ErrNoFileJournal
		}
		return replayed, errorValue
	}
}
