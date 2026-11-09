package filestream

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/box"
	"github.com/yeomyeonggeori/internkim/internal/hostbackup"
	"github.com/yeomyeonggeori/internkim/internal/offsite"
)

const (
	entryVersion     = 1
	journalFolder    = "file-journal"
	fileChunkFolder  = "file-chunks"
	entryNameLayout  = "%020d"
	entryContentName = "files"
)

var ErrNoFullEntry = errors.New("the company keeps no full file journal entry this box can start from")

type Entry struct {
	Version   int       `json:"version"`
	Sequence  int64     `json:"sequence"`
	CreatedAt time.Time `json:"createdAt"`
	IsFull    bool      `json:"isFull"`
	Roles     []string  `json:"roles"`
	Removed   []string  `json:"removed"`
	Chunks    []string  `json:"chunks"`
	Size      int64     `json:"size"`
	SHA256    string    `json:"sha256"`
}

func (entry Entry) name() string {
	return fmt.Sprintf(entryNameLayout, entry.Sequence)
}

type Journal struct {
	Destination offsite.Destination
}

func (journal Journal) chunkDestination() offsite.Destination {
	destination := journal.Destination
	destination.ChunkFolder = fileChunkFolder
	return destination
}

func (journal Journal) folder() string {
	return journal.Destination.CompanyID + "/" + journalFolder + "/"
}

func (journal Journal) Keep(ctx context.Context, entry Entry) error {
	document, errorValue := json.Marshal(entry)
	if errorValue != nil {
		return errorValue
	}
	var sealed bytes.Buffer
	sealing, errorValue := box.SealStreamTo(&sealed, journal.Destination.SealingKey, box.FileJournalPurpose(journal.Destination.CompanyID, entry.name()))
	if errorValue != nil {
		return errorValue
	}
	if _, errorValue := sealing.Write(document); errorValue != nil {
		return errorValue
	}
	if errorValue := sealing.Close(); errorValue != nil {
		return errorValue
	}
	if errorValue := journal.Destination.Store.Put(ctx, journal.folder()+entry.name(), sealed.Bytes()); errorValue != nil {
		return fmt.Errorf("keeping the file journal entry %d: %w", entry.Sequence, errorValue)
	}
	return nil
}

func (journal Journal) Sequences(ctx context.Context) ([]int64, error) {
	listed, errorValue := journal.Destination.Store.List(ctx, journal.folder())
	if errorValue != nil {
		return nil, errorValue
	}
	sequences := make([]int64, 0, len(listed))
	for _, object := range listed {
		sequence, errorValue := strconv.ParseInt(object.Name, 10, 64)
		if errorValue != nil || fmt.Sprintf(entryNameLayout, sequence) != object.Name {
			return nil, fmt.Errorf("the company's file journal holds %q, which is not an entry name", object.Name)
		}
		sequences = append(sequences, sequence)
	}
	return sequences, nil
}

func (journal Journal) Read(ctx context.Context, sequence int64, keyring box.Keyring) (Entry, error) {
	name := fmt.Sprintf(entryNameLayout, sequence)
	sealed, errorValue := journal.Destination.Store.Get(ctx, journal.folder()+name)
	if errorValue != nil {
		return Entry{}, fmt.Errorf("reading the file journal entry %d: %w", sequence, errorValue)
	}
	opened, errorValue := keyring.OpenStream(bytes.NewReader(sealed), box.FileJournalPurpose(journal.Destination.CompanyID, name))
	if errorValue != nil {
		return Entry{}, fmt.Errorf("opening the file journal entry %d: %w", sequence, errorValue)
	}
	var entry Entry
	if errorValue := json.NewDecoder(opened).Decode(&entry); errorValue != nil {
		return Entry{}, fmt.Errorf("opening the file journal entry %d: %w", sequence, errorValue)
	}
	if entry.Version != entryVersion || entry.Sequence != sequence {
		return Entry{}, fmt.Errorf("the file journal entry %d is version %d numbered %d, which this box does not read", sequence, entry.Version, entry.Sequence)
	}
	return entry, nil
}

func (journal Journal) sinceNewestFull(ctx context.Context, keyring box.Keyring) ([]Entry, error) {
	sequences, errorValue := journal.Sequences(ctx)
	if errorValue != nil {
		return nil, errorValue
	}
	entries := []Entry{}
	for index := len(sequences) - 1; index >= 0; index-- {
		entry, errorValue := journal.Read(ctx, sequences[index], keyring)
		if errorValue != nil {
			return nil, errorValue
		}
		entries = append([]Entry{entry}, entries...)
		if entry.IsFull {
			return entries, nil
		}
	}
	return nil, ErrNoFullEntry
}

func (journal Journal) Replay(ctx context.Context, keyring box.Keyring, cache offsite.ChunkCache, apply func(Entry, io.Reader) error) (int, error) {
	entries, errorValue := journal.sinceNewestFull(ctx, keyring)
	if errorValue != nil {
		return 0, errorValue
	}
	for _, entry := range entries {
		if errorValue := journal.replayOne(ctx, keyring, cache, entry, apply); errorValue != nil {
			return 0, errorValue
		}
	}
	return len(entries), nil
}

func (journal Journal) replayOne(ctx context.Context, keyring box.Keyring, cache offsite.ChunkCache, entry Entry, apply func(Entry, io.Reader) error) error {
	member := hostbackup.Member{Name: entryContentName, Size: entry.Size, SHA256: entry.SHA256}
	manifest := offsite.Manifest{Name: entry.name(), Chunks: map[string][]string{entryContentName: entry.Chunks}}
	reading, writing := io.Pipe()
	go func() {
		writing.CloseWithError(cache.WriteMember(ctx, journal.chunkDestination(), manifest, member, keyring, writing))
	}()
	errorValue := apply(entry, reading)
	if errorValue == nil {
		_, errorValue = io.Copy(io.Discard, reading)
	}
	reading.CloseWithError(errorValue)
	if errorValue != nil {
		return fmt.Errorf("replaying the file journal entry %d: %w", entry.Sequence, errorValue)
	}
	return nil
}
