package admind

import (
	"context"
	"strings"
	"sync"
	"time"
)

var (
	mirrorTriggerMutex   sync.Mutex
	mirrorTriggerLastAt  time.Time
	mirrorTriggerRunning bool
)

const mirrorTriggerThrottle = 20 * time.Second

// The messenger reads from Buzz. The real-time chatd Buzz<->Mattermost mirror
// now carries new Mattermost messages into Buzz per-user, so the older
// on-channel-open incremental importer is disabled: running both double-mirrors
// Mattermost messages and the importer's orphan-thread roots surface as empty
// "이전 대화" bubbles. Kept as a no-op guard so callers stay compiled.
var mattermostMirrorImporterDisabled = true

func (service *Service) triggerMattermostMirror() {
	if mattermostMirrorImporterDisabled {
		return
	}
	if strings.TrimSpace(service.buzzKeySeed()) == "" {
		return
	}
	mirrorTriggerMutex.Lock()
	if mirrorTriggerRunning || time.Since(mirrorTriggerLastAt) < mirrorTriggerThrottle {
		mirrorTriggerMutex.Unlock()
		return
	}
	mirrorTriggerRunning = true
	mirrorTriggerLastAt = time.Now()
	mirrorTriggerMutex.Unlock()

	go service.runMattermostMirror()
}

func (service *Service) runMattermostMirror() {
	defer func() {
		mirrorTriggerMutex.Lock()
		mirrorTriggerRunning = false
		mirrorTriggerMutex.Unlock()
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	service.runCommand(ctx, "systemctl", "start", "buzz-mirror-sync.service")
}
