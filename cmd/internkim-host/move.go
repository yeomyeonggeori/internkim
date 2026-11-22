package main

import (
	"context"
	"fmt"

	"github.com/jackc/pglogrepl"

	"github.com/yeomyeonggeori/internkim/internal/box"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
	"github.com/yeomyeonggeori/internkim/internal/walstream"
)

func whereTheMoveStopped(claim box.Session) (*box.LogPosition, error) {
	identity, errorValue := box.LoadIdentity(blueclaw.CompanyHostBoxStatePath)
	if errorValue != nil {
		return nil, errorValue
	}
	move := claim.MoveTo(identity.PublicKey())
	if move == nil {
		return nil, nil
	}
	if move.Stop == nil {
		return nil, fmt.Errorf("%s moves to this box once box %s has stopped and kept what it holds, and it has not yet", claim.Configuration.Company.Name, move.FromBox)
	}
	return move.Stop, nil
}

func walThrough(stop *box.LogPosition) (walstream.Position, error) {
	if stop == nil {
		return walstream.Position{}, nil
	}
	position, errorValue := pglogrepl.ParseLSN(stop.WALPosition)
	if errorValue != nil {
		return walstream.Position{}, fmt.Errorf("the box the company moves from stopped at WAL %q, which is not a WAL position: %w", stop.WALPosition, errorValue)
	}
	return walstream.Position{Timeline: stop.WALTimeline, LSN: position}, nil
}

func fileEntryThrough(stop *box.LogPosition) *int64 {
	if stop == nil {
		return nil
	}
	return stop.FileEntry
}

func finishingTheMoveFirst(stop *box.LogPosition, thenStart func(companyID string) error) func(companyID string) error {
	if stop == nil {
		return thenStart
	}
	return func(companyID string) error {
		if errorValue := reportCaughtUp(*stop); errorValue != nil {
			return errorValue
		}
		return thenStart(companyID)
	}
}

func reportCaughtUp(stop box.LogPosition) error {
	identity, errorValue := box.LoadIdentity(blueclaw.CompanyHostBoxStatePath)
	if errorValue != nil {
		return errorValue
	}
	appURL, errorValue := appURLThisBoxAnswersTo()
	if errorValue != nil {
		return errorValue
	}
	outcome, errorValue := box.Client{AppURL: appURL}.ReportMove(context.Background(), identity, stop)
	if errorValue != nil {
		return fmt.Errorf("telling the central plane this box replayed the company's logs through %s: %w", stop, errorValue)
	}
	if outcome != box.MoveServing {
		return fmt.Errorf("this box replayed the company's logs through %s, and the central plane answered %s instead of making it serve", stop, outcome)
	}
	return nil
}
