package capabilityd

import (
	"sync"

	"github.com/yeomyeonggeori/internkim/internal/desktop"
)

const longestObservedWindowMemory = 256

type observedSnapshot struct {
	desktopOwner string
	snapshotID   string
}

type observedWindows struct {
	mutex           sync.Mutex
	scopeBySnapshot map[observedSnapshot]desktop.Scope
	oldestFirst     []observedSnapshot
}

func (observed *observedWindows) scopeOf(desktopOwner string, snapshotID string) desktop.Scope {
	if observed == nil {
		return desktop.Scope{}
	}
	observed.mutex.Lock()
	defer observed.mutex.Unlock()
	return observed.scopeBySnapshot[observedSnapshot{desktopOwner, snapshotID}]
}

func (observed *observedWindows) remember(desktopOwner string, snapshotID string, scope desktop.Scope) {
	if observed == nil || snapshotID == "" || scope == (desktop.Scope{}) {
		return
	}
	observed.mutex.Lock()
	defer observed.mutex.Unlock()
	snapshot := observedSnapshot{desktopOwner, snapshotID}
	if _, isRemembered := observed.scopeBySnapshot[snapshot]; isRemembered {
		return
	}
	if observed.scopeBySnapshot == nil {
		observed.scopeBySnapshot = map[observedSnapshot]desktop.Scope{}
	}
	observed.scopeBySnapshot[snapshot] = scope
	observed.oldestFirst = append(observed.oldestFirst, snapshot)
	if len(observed.oldestFirst) > longestObservedWindowMemory {
		delete(observed.scopeBySnapshot, observed.oldestFirst[0])
		observed.oldestFirst = observed.oldestFirst[1:]
	}
}
