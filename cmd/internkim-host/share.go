package main

import (
	"context"
	"errors"
	"log"
	"slices"
	"sync"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/box"
	"github.com/yeomyeonggeori/internkim/internal/filestream"
	"github.com/yeomyeonggeori/internkim/internal/offsite"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
	"github.com/yeomyeonggeori/internkim/internal/walstream"
)

const shareKeptEvery = 10 * time.Second

var logFolders = slices.Concat(walstream.LogFolders, filestream.LogFolders)

func thisBoxShare() offsite.Share {
	return offsite.Share{DirectoryPath: blueclaw.CompanyHostStandbyChunksPath}
}

type shareKeeper struct {
	mutex     sync.Mutex
	keeping   box.ShareKeeping
	isRunning bool
}

var keeper shareKeeper

func keepThisBoxShare(ctx context.Context, keeping box.ShareKeeping) {
	if keeper.isStartedBy(keeping) {
		go keeper.keep(ctx)
	}
}

func (keeper *shareKeeper) isStartedBy(keeping box.ShareKeeping) bool {
	keeper.mutex.Lock()
	defer keeper.mutex.Unlock()
	keeper.keeping = keeping
	if keeper.isRunning {
		return false
	}
	keeper.isRunning = true
	return true
}

func (keeper *shareKeeper) latest() box.ShareKeeping {
	keeper.mutex.Lock()
	defer keeper.mutex.Unlock()
	return keeper.keeping
}

func (keeper *shareKeeper) stop() {
	keeper.mutex.Lock()
	defer keeper.mutex.Unlock()
	keeper.isRunning = false
}

func (keeper *shareKeeper) keep(ctx context.Context) {
	defer keeper.stop()
	for {
		errorValue := keepTheShareOnce(ctx, keeper.latest())
		if errors.Is(errorValue, box.ErrBoxOfNoCompany) {
			return
		}
		if errorValue != nil {
			log.Printf("keeping this box's share of the company's log: %v; trying again in %s", errorValue, shareKeptEvery)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(shareKeptEvery):
		}
	}
}

func keepTheShareOnce(ctx context.Context, keeping box.ShareKeeping) error {
	if keeping.IsStartingTheCompany {
		return nil
	}
	placement, errorValue := keeping.Report(ctx, nil)
	if errorValue != nil {
		return errorValue
	}
	report, errorValue := thisBoxShare().KeepUp(ctx, companyDestinationOf(keeping.Session), logFolders, keeping.PublicKey, placement)
	if errorValue != nil {
		return errorValue
	}
	_, errorValue = keeping.Report(ctx, &report)
	return errorValue
}
