package main

import (
	"context"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/box"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

const severalBoxesCheckEvery = 15 * time.Second

func companyRunsOnSeveralBoxes() bool {
	return box.RunsOnSeveralBoxes(blueclaw.CompanyHostSeveralBoxesPath)
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
				if companyRunsOnSeveralBoxes() != hasSeveralBoxes {
					cancel()
					return
				}
			}
		}
	}()
	return watched, cancel
}
