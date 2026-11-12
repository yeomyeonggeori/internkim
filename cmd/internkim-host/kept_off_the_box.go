package main

import (
	"context"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/box"
	"github.com/yeomyeonggeori/internkim/internal/offsite"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

const keptOffTheBoxCheckEvery = 15 * time.Second

type keptOffTheBox struct {
	destination        offsite.Destination
	keyring            box.Keyring
	holdsTheCompanyKey bool
	isJournaled        bool
}

func whatThisBoxKeeps() (keptOffTheBox, error) {
	return whatIsKeptOffTheBox(thisBox(), blueclaw.CompanyHostSeveralBoxesPath)
}

func whatIsKeptOffTheBox(place offsite.ThisBox, severalBoxesPath string) (keptOffTheBox, error) {
	destination, keyring, holdsTheCompanyKey, errorValue := place.Destination()
	if errorValue != nil {
		return keptOffTheBox{}, errorValue
	}
	hasSeveralBoxes, _ := box.CompanyBoxCount(severalBoxesPath)
	return keptOffTheBox{
		destination:        destination,
		keyring:            keyring,
		holdsTheCompanyKey: holdsTheCompanyKey,
		isJournaled:        holdsTheCompanyKey && hasSeveralBoxes,
	}, nil
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
				if now, errorValue := whatThisBoxKeeps(); errorValue != nil || now.isJournaled != kept.isJournaled {
					cancel()
					return
				}
			}
		}
	}()
	return watched, cancel
}
