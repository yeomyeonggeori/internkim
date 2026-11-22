package box

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"
)

var ErrBoxOfNoCompany = errors.New("this box belongs to no company")

type ShareReport struct {
	RankedAgainst string     `json:"rankedAgainst"`
	KeptThrough   *time.Time `json:"keptThrough"`
	SharePieces   int        `json:"sharePieces"`
	HeldPieces    int        `json:"heldPieces"`
	LogPieces     int        `json:"logPieces"`
}

type ShareHolder struct {
	PublicKey   string     `json:"publicKey"`
	KeptThrough *time.Time `json:"keptThrough"`
}

type SharePlacement struct {
	StandbysDigest string        `json:"standbysDigest"`
	Standbys       []ShareHolder `json:"standbys"`
}

type ShareKeeping struct {
	Session              Session
	PublicKey            string
	IsStartingTheCompany bool
	Report               func(ctx context.Context, report *ShareReport) (SharePlacement, error)
}

func (client Client) ReportShare(ctx context.Context, identity Identity, report *ShareReport) (SharePlacement, error) {
	body, errorValue := json.Marshal(map[string]*ShareReport{"report": report})
	if errorValue != nil {
		return SharePlacement{}, errorValue
	}
	response, errorValue := client.post(ctx, identity, "/api/box/share", body)
	if errorValue != nil {
		return SharePlacement{}, errorValue
	}
	defer response.Body.Close()
	if response.StatusCode == http.StatusNotFound {
		return SharePlacement{}, ErrBoxOfNoCompany
	}
	if response.StatusCode != http.StatusOK {
		return SharePlacement{}, refusalOf(response, "reporting the share of the log this box holds")
	}
	var placement SharePlacement
	if errorValue := json.NewDecoder(response.Body).Decode(&placement); errorValue != nil {
		return SharePlacement{}, fmt.Errorf("reporting the share of the log this box holds: %w", errorValue)
	}
	return placement, nil
}

func (daemon Daemon) keepTheShare(ctx context.Context, session Session, identity Identity) {
	if daemon.KeepTheShare == nil {
		return
	}
	daemon.KeepTheShare(ctx, ShareKeeping{
		Session:              session,
		PublicKey:            identity.PublicKey(),
		IsStartingTheCompany: session.Role == RoleService && daemon.installedCompany() != session.Configuration.Company.ID,
		Report: func(reporting context.Context, report *ShareReport) (SharePlacement, error) {
			return daemon.Client.ReportShare(reporting, identity, report)
		},
	})
}
