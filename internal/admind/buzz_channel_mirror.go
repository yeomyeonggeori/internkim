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

// The messenger reads from Buzz. Instead of polling Mattermost on a background
// timer, an incremental import is kicked off when someone opens or refreshes a
// channel. A throttle collapses a burst of conversation loads (the client polls
// while a channel is open) into at most one sync, and only one runs at a time.
func (service *Service) triggerMattermostMirror() {
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
