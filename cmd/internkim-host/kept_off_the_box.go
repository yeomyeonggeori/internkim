package main

import (
	"context"
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

func whileThisBoxKeepsTheSame(ctx context.Context, kept keptOffTheBox) (context.Context, context.CancelFunc) {
	watched, cancel := context.WithCancel(ctx)
	go func() {
		ticker := time.NewTicker(keptOffTheBoxCheckEvery)
		defer ticker.Stop()
		for {
			select {
			case <-watched.Done():
				return
			case <-ticker.C:
				if now, errorValue := whatThisBoxKeeps(); errorValue != nil || now.holdsTheCompanyKey != kept.holdsTheCompanyKey {
					cancel()
					return
				}
			}
		}
	}()
	return watched, cancel
}
