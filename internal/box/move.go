package box

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/durablefile"
)

const moveCheckInterval = 2 * time.Second

type LogPosition struct {
	WALTimeline int32  `json:"walTimeline"`
	WALPosition string `json:"walPosition"`
	FileEntry   *int64 `json:"fileEntry"`
}

func (position LogPosition) String() string {
	if position.FileEntry == nil {
		return fmt.Sprintf("WAL %s on timeline %d, with no file journal", position.WALPosition, position.WALTimeline)
	}
	return fmt.Sprintf("WAL %s on timeline %d and file journal entry %d", position.WALPosition, position.WALTimeline, *position.FileEntry)
}

type Move struct {
	FromBox string       `json:"fromBox"`
	ToBox   string       `json:"toBox"`
	Stop    *LogPosition `json:"stop"`
}

type MoveOutcome string

const (
	MoveStopped MoveOutcome = "stopped"
	MoveServing MoveOutcome = "serving"
)

func (session Session) isMovingFrom(publicKey string) bool {
	return session.Move != nil && session.Move.FromBox == publicKey
}

func (session Session) MoveTo(publicKey string) *Move {
	if session.Move == nil || session.Move.ToBox != publicKey {
		return nil
	}
	return session.Move
}

func (client Client) ReportMove(ctx context.Context, identity Identity, position LogPosition) (MoveOutcome, error) {
	body, errorValue := json.Marshal(position)
	if errorValue != nil {
		return "", errorValue
	}
	response, errorValue := client.post(ctx, identity, "/api/box/move", body)
	if errorValue != nil {
		return "", errorValue
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", refusalOf(response, "telling the central plane where the company's logs end for the move")
	}
	var answer struct {
		Outcome MoveOutcome `json:"outcome"`
	}
	if errorValue := json.NewDecoder(response.Body).Decode(&answer); errorValue != nil {
		return "", fmt.Errorf("telling the central plane where the company's logs end for the move: %w", errorValue)
	}
	return answer.Outcome, nil
}

func (daemon Daemon) handOver(ctx context.Context, session Session, identity Identity) error {
	if session.Move.Stop != nil || daemon.HandOver == nil {
		return nil
	}
	log.Printf("an administrator moves %s to box %s, so this box stops its services and reports where the company's logs end in the store", session.Configuration.Company.Name, session.Move.ToBox)
	daemon.forgetTheQueueAddress()
	if errorValue := daemon.noteStoodDown("moving to " + session.Move.ToBox); errorValue != nil {
		return errorValue
	}
	stop, errorValue := daemon.HandOver(ctx, session)
	if errorValue != nil {
		return fmt.Errorf("stopping %s's services for the move: %w", session.Configuration.Company.Name, errorValue)
	}
	outcome, errorValue := daemon.Client.ReportMove(ctx, identity, stop)
	if errorValue != nil {
		return errorValue
	}
	log.Printf("this box stopped with %s kept in the store; the central plane answered %s", stop, outcome)
	return nil
}

func (daemon Daemon) catchUp(ctx context.Context, session Session, keyring Keyring) (time.Duration, error) {
	if session.Move.Stop == nil {
		log.Printf("%s moves to this box once box %s has stopped and kept what it holds", session.Configuration.Company.Name, session.Move.FromBox)
		return moveCheckInterval, nil
	}
	_, errorValue := daemon.takeOver(ctx, session, keyring)
	return moveCheckInterval, errorValue
}

func (daemon Daemon) noteStoodDown(reason string) error {
	return durablefile.Write(daemon.statePath(stoodDownFileName), []byte(reason+"\n"), 0o600)
}
