package capabilityd

import (
	"sync"
	"time"
)

type pageRequestHandover interface {
	HandOver()
	TakeBack()
}

type openHandoff struct {
	handoffID    string
	requester    string
	signInOrigin string
	cookieDigest string
	expiresAt    time.Time
	guard        pageRequestHandover
}

const endedHandoffsKept = 64

type browserHandoffs struct {
	mutex       sync.Mutex
	byRequester map[string]openHandoff
	endedByID   map[string]openHandoff
	endedOrder  []string
}

func (handoffs *browserHandoffs) isHandedOver(requester string) bool {
	_, isOpen := handoffs.openFor(requester)
	return isOpen
}

func (handoffs *browserHandoffs) openFor(requester string) (openHandoff, bool) {
	if handoffs == nil {
		return openHandoff{}, false
	}
	handoffs.mutex.Lock()
	defer handoffs.mutex.Unlock()
	handoff, isOpen := handoffs.byRequester[requester]
	return handoff, isOpen
}

func (handoffs *browserHandoffs) open(handoff openHandoff) {
	handoffs.mutex.Lock()
	defer handoffs.mutex.Unlock()
	if handoffs.byRequester == nil {
		handoffs.byRequester = map[string]openHandoff{}
	}
	handoffs.byRequester[handoff.requester] = handoff
	handoff.guard.HandOver()
}

func (handoffs *browserHandoffs) end(requester string, handoffID string) (openHandoff, bool) {
	if handoffs == nil {
		return openHandoff{}, false
	}
	handoffs.mutex.Lock()
	defer handoffs.mutex.Unlock()
	handoff, isOpen := handoffs.byRequester[requester]
	if !isOpen || handoff.handoffID != handoffID {
		return openHandoff{}, false
	}
	delete(handoffs.byRequester, requester)
	handoff.guard.TakeBack()
	handoffs.keepEnded(handoff)
	return handoff, true
}

func (handoffs *browserHandoffs) keepEnded(handoff openHandoff) {
	if handoffs.endedByID == nil {
		handoffs.endedByID = map[string]openHandoff{}
	}
	for len(handoffs.endedOrder) >= endedHandoffsKept {
		delete(handoffs.endedByID, handoffs.endedOrder[0])
		handoffs.endedOrder = handoffs.endedOrder[1:]
	}
	handoffs.endedByID[handoff.handoffID] = handoff
	handoffs.endedOrder = append(handoffs.endedOrder, handoff.handoffID)
}

func (handoffs *browserHandoffs) takeEnded(requester string, handoffID string) (openHandoff, bool) {
	if handoffs == nil {
		return openHandoff{}, false
	}
	handoffs.mutex.Lock()
	defer handoffs.mutex.Unlock()
	handoff, isKept := handoffs.endedByID[handoffID]
	if !isKept || handoff.requester != requester {
		return openHandoff{}, false
	}
	delete(handoffs.endedByID, handoffID)
	return handoff, true
}
