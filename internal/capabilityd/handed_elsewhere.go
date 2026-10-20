package capabilityd

import "sync"

type handedElsewhere struct {
	mutex      sync.Mutex
	requesters map[string]bool
}

func (handed *handedElsewhere) has(requester string) bool {
	if handed == nil {
		return false
	}
	handed.mutex.Lock()
	defer handed.mutex.Unlock()
	return handed.requesters[requester]
}

func (handed *handedElsewhere) remember(requester string) {
	if handed == nil {
		return
	}
	handed.mutex.Lock()
	defer handed.mutex.Unlock()
	if handed.requesters == nil {
		handed.requesters = map[string]bool{}
	}
	handed.requesters[requester] = true
}

func (handed *handedElsewhere) forget(requester string) {
	if handed == nil {
		return
	}
	handed.mutex.Lock()
	defer handed.mutex.Unlock()
	delete(handed.requesters, requester)
}
