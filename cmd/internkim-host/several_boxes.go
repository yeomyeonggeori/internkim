package main

import (
	"context"
	"log"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/box"
	"github.com/yeomyeonggeori/internkim/internal/offsite"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

const severalBoxesCheckEvery = 15 * time.Second

func companyBoxCount() (bool, bool) {
	return box.CompanyBoxCount(blueclaw.CompanyHostSeveralBoxesPath)
}

func untilThisBoxServes(ctx context.Context, place offsite.ThisBox) (offsite.Destination, box.Keyring, error) {
	isWaiting := false
	for {
		destination, keyring, isServing, errorValue := place.Destination()
		if errorValue != nil || isServing {
			return destination, keyring, errorValue
		}
		if !isWaiting {
			log.Print("this box holds no company key yet, so nothing is kept for the company's other boxes until it does")
			isWaiting = true
		}
		select {
		case <-ctx.Done():
			return offsite.Destination{}, box.Keyring{}, ctx.Err()
		case <-time.After(time.Second):
		}
	}
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
