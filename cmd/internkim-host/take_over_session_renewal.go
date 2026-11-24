package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/box"
	"github.com/yeomyeonggeori/internkim/internal/companyhost"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

const (
	takeOverSessionRetryWait    = time.Minute
	takeOverSessionAttempts     = 3
	takeOverSessionFirstBackoff = 2 * time.Second
)

type takeOverSession struct {
	Path string
	Ask  func(context.Context) (box.Session, error)
}

func (session takeOverSession) renew(ctx context.Context) (box.Session, error) {
	asked, errorValue := session.Ask(ctx)
	if errorValue != nil {
		return box.Session{}, fmt.Errorf("asking for the session this box finishes the take-over with: %w", errorValue)
	}
	if errorValue := keepTheTakeOverSession(session.Path, asked.Session.AccessToken); errorValue != nil {
		return box.Session{}, fmt.Errorf("keeping the session this box finishes the take-over with: %w", errorValue)
	}
	return asked, nil
}

func (session takeOverSession) keepFresh(ctx context.Context) {
	wait := time.Duration(0)
	for {
		select {
		case <-ctx.Done():
			return
		case <-time.After(wait):
		}
		renewed, errorValue := session.renew(ctx)
		if errorValue != nil {
			log.Printf("%v; trying again in %s", errorValue, takeOverSessionRetryWait)
			wait = takeOverSessionRetryWait
			continue
		}
		wait = untilTheNextRenewal(renewed)
	}
}

func untilTheNextRenewal(renewed box.Session) time.Duration {
	untilTheSessionRunsOut := max(time.Until(time.Unix(renewed.Session.ExpiresAt, 0))-box.SessionRenewalMargin, takeOverSessionRetryWait)
	return min(untilTheSessionRunsOut, box.UntilTheTakeOverNeedsACheck(renewed))
}

func (session takeOverSession) fresh() (string, error) {
	backoff := takeOverSessionFirstBackoff
	for attempt := 1; ; attempt++ {
		renewed, errorValue := session.renew(context.Background())
		if errorValue == nil {
			return renewed.Session.AccessToken, nil
		}
		if attempt == takeOverSessionAttempts {
			return "", fmt.Errorf("%w; the company's services were not started, and this box tries the take-over again by itself", errorValue)
		}
		log.Printf("%v; trying again in %s", errorValue, backoff)
		time.Sleep(backoff)
		backoff *= 2
	}
}

func confirmingTheLeaseOf(ask func(context.Context) (box.Session, error), boxStatePath string) func(context.Context) (box.Session, error) {
	return func(ctx context.Context) (box.Session, error) {
		claim, errorValue := ask(ctx)
		if errorValue != nil || claim.Role != box.RoleService {
			return claim, errorValue
		}
		return claim, box.ConfirmTheLease(boxStatePath, time.Now())
	}
}

func askingForThisBoxClaim() (func(context.Context) (box.Session, error), error) {
	identity, errorValue := box.LoadIdentity(blueclaw.CompanyHostBoxStatePath)
	if errorValue != nil {
		return nil, errorValue
	}
	appURL, errorValue := appURLThisBoxAnswersTo()
	if errorValue != nil {
		return nil, errorValue
	}
	client := box.Client{AppURL: appURL}
	return func(ctx context.Context) (box.Session, error) {
		session, isClaimed, errorValue := client.Session(ctx, identity, thisBoxRelease())
		if errorValue != nil {
			return box.Session{}, errorValue
		}
		if !isClaimed {
			return box.Session{}, fmt.Errorf("no company holds this box any longer")
		}
		return session, nil
	}, nil
}

func appURLThisBoxAnswersTo() (string, error) {
	if remembered := box.RememberedAppURL(blueclaw.CompanyHostBoxStatePath); remembered != "" {
		return remembered, nil
	}
	connection, errorValue := companyhost.ReadConnection(companyhost.CurrentConnectionPath())
	if errorValue != nil {
		return "", fmt.Errorf("this box remembers no address to ask for its session at, and its connection could not be read: %w", errorValue)
	}
	if connection.AppURL == "" {
		return "", fmt.Errorf("this box remembers no address to ask for its session at, and %s names none", companyhost.CurrentConnectionPath())
	}
	return connection.AppURL, nil
}
