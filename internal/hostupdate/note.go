package hostupdate

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

var NotePath = filepath.Join(blueclaw.CompanyHostStateRoot, "host-update.json")

type Requester struct {
	Email          string `json:"email"`
	PersonID       string `json:"personID,omitempty"`
	Platform       string `json:"platform,omitempty"`
	ConversationID string `json:"conversationID,omitempty"`
	ReplyTargetID  string `json:"replyTargetID,omitempty"`
}

type Outcome struct {
	FinishedAt time.Time `json:"finishedAt"`
	Succeeded  bool      `json:"succeeded"`
	Error      string    `json:"error,omitempty"`
	OutputTail string    `json:"outputTail,omitempty"`
}

type Note struct {
	Requester   Requester `json:"requester"`
	FromVersion string    `json:"fromVersion"`
	ToVersion   string    `json:"toVersion"`
	StartedAt   time.Time `json:"startedAt"`
	Outcome     *Outcome  `json:"outcome,omitempty"`
}

func (note Note) IsFinished() bool {
	return note.Outcome != nil
}

func ReadNote(path string) (Note, bool, error) {
	contents, errorValue := os.ReadFile(path)
	if errors.Is(errorValue, os.ErrNotExist) {
		return Note{}, false, nil
	}
	if errorValue != nil {
		return Note{}, false, errorValue
	}
	var note Note
	if errorValue := json.Unmarshal(contents, &note); errorValue != nil {
		return Note{}, false, errorValue
	}
	return note, true, nil
}

func WriteNote(path string, note Note) error {
	contents, errorValue := json.MarshalIndent(note, "", "  ")
	if errorValue != nil {
		return errorValue
	}
	temporaryPath := path + ".writing"
	if errorValue := os.WriteFile(temporaryPath, contents, 0o600); errorValue != nil {
		return errorValue
	}
	return os.Rename(temporaryPath, path)
}

func ClearNote(path string) error {
	errorValue := os.Remove(path)
	if errors.Is(errorValue, os.ErrNotExist) {
		return nil
	}
	return errorValue
}
