package main

import (
	"context"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/box"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

const severalBoxesCheckEvery = 15 * time.Second

func companyBoxCount() (bool, bool) {
	return box.CompanyBoxCount(blueclaw.CompanyHostSeveralBoxesPath)
}

func untilTheBoxCountIsKnown(ctx context.Context) (bool, error) {
	for {
		if hasSeveralBoxes, isKnown := companyBoxCount(); isKnown {
			return hasSeveralBoxes, nil
		}
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		case <-time.After(time.Second):
		}
	}
}

func whileSeveralBoxesIs(ctx context.Context, hasSeveralBoxes bool) (context.Context, context.CancelFunc) {
	watched, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(severalBoxesCheckEvery)
		defer ticker.Stop()
		for {
			select {
			case <-watched.Done():
				return
			case <-ticker.C:
				if isSeveral, isKnown := companyBoxCount(); isKnown && isSeveral != hasSeveralBoxes {
					cancel()
					return
				}
			}
		}
	}()
	return watched, cancel
}
