package main

import (
	"context"
	"errors"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/box"
	"github.com/yeomyeonggeori/internkim/internal/offsite"
)

const keptOffTheBoxCheckEvery = 15 * time.Second

type keptOffTheBox struct {
	destination        offsite.Destination
	keyring            box.Keyring
	holdsTheCompanyKey bool
}

type keepingOffTheBox func(outer, watched context.Context, kept keptOffTheBox) error

func whatThisBoxKeeps() (keptOffTheBox, error) {
	return whatIsKeptOffTheBox(thisBox())
}

func whatIsKeptOffTheBox(place offsite.ThisBox) (keptOffTheBox, error) {
	destination, keyring, holdsTheCompanyKey, errorValue := place.Destination()
	if errorValue != nil {
		return keptOffTheBox{}, errorValue
	}
	return keptOffTheBox{destination: destination, keyring: keyring, holdsTheCompanyKey: holdsTheCompanyKey}, nil
}

func keepWhatThisBoxKeeps(ctx context.Context, keep keepingOffTheBox) error {
	return keepWhatTheBoxKeeps(ctx, thisBox(), keptOffTheBoxCheckEvery, keep)
}

func keepWhatTheBoxKeeps(ctx context.Context, place offsite.ThisBox, checkEvery time.Duration, keep keepingOffTheBox) error {
	for {
		kept, errorValue := whatIsKeptOffTheBox(place)
		if errorValue != nil {
			return errorValue
		}
		watched, cancel := whileTheBoxKeepsTheSame(ctx, place, checkEvery, kept)
		errorValue = keep(ctx, watched, kept)
		cancel()
		if ctx.Err() != nil || !isEndedByKeepingSomethingElse(watched, errorValue) {
			return errorValue
		}
	}
}

func isEndedByKeepingSomethingElse(watched context.Context, errorValue error) bool {
	return watched.Err() != nil && (errorValue == nil || errors.Is(errorValue, context.Canceled))
}

func whileTheBoxKeepsTheSame(ctx context.Context, place offsite.ThisBox, checkEvery time.Duration, kept keptOffTheBox) (context.Context, context.CancelFunc) {
	watched, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(checkEvery)
		defer ticker.Stop()
		for {
			select {
			case <-watched.Done():
				return
			case <-ticker.C:
				if now, errorValue := whatIsKeptOffTheBox(place); errorValue != nil || now.holdsTheCompanyKey != kept.holdsTheCompanyKey {
					cancel()
					return
				}
			}
		}
	}()
	return watched, cancel
}
