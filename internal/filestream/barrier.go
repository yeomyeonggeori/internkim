package filestream

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	BarrierDirectoryName = ".file-journal-barrier"
	markerBytes          = 16
)

type Barrier struct {
	Root    string
	Role    string
	mutex   sync.Mutex
	waiting map[string]chan struct{}
}

func NewBarrier(rootPath, role string) *Barrier {
	return &Barrier{Root: rootPath, Role: role, waiting: map[string]chan struct{}{}}
}

func (barrier *Barrier) directoryPath() string {
	return filepath.Join(barrier.Root, BarrierDirectoryName)
}

func (barrier *Barrier) Prepare() error {
	return os.MkdirAll(barrier.directoryPath(), 0o700)
}

func (barrier *Barrier) Wait(ctx context.Context) error {
	random := make([]byte, markerBytes)
	if _, errorValue := rand.Read(random); errorValue != nil {
		return errorValue
	}
	marker := hex.EncodeToString(random)
	released := make(chan struct{})
	barrier.mutex.Lock()
	barrier.waiting[marker] = released
	barrier.mutex.Unlock()
	defer barrier.forget(marker)
	markerPath := filepath.Join(barrier.directoryPath(), marker)
	if errorValue := os.WriteFile(markerPath, nil, 0o600); errorValue != nil {
		return fmt.Errorf("asking for the company's files to be kept: %w", errorValue)
	}
	defer os.Remove(markerPath)
	select {
	case <-released:
		return nil
	case <-ctx.Done():
		return fmt.Errorf("the company's files were not kept off the box in time: %w", ctx.Err())
	}
}

func isBarrierPath(relative string) bool {
	return relative == BarrierDirectoryName || strings.HasPrefix(relative, BarrierDirectoryName+"/")
}

func (barrier *Barrier) forget(marker string) {
	barrier.mutex.Lock()
	delete(barrier.waiting, marker)
	barrier.mutex.Unlock()
}

func (barrier *Barrier) markerOf(change Change) (string, bool) {
	if barrier == nil || change.Root.Role != barrier.Role {
		return "", false
	}
	marker, isMarker := strings.CutPrefix(change.Relative, BarrierDirectoryName+"/")
	if !isMarker {
		return "", change.Relative == BarrierDirectoryName
	}
	return marker, true
}

func (barrier *Barrier) release(markers []string) {
	if barrier == nil {
		return
	}
	barrier.mutex.Lock()
	defer barrier.mutex.Unlock()
	for _, marker := range markers {
		if released, isWaiting := barrier.waiting[marker]; isWaiting {
			close(released)
			delete(barrier.waiting, marker)
		}
	}
}
