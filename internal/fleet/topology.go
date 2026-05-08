package fleet

import (
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"sort"
	"strings"
)

const (
	MemberStatusActive  = "active"
	MemberStatusPending = "pending"
)

type Member struct {
	NodeID string
	Status string
}

type State struct {
	FleetID string
	Members []Member
}

type JoinResult struct {
	State            State
	Member           Member
	ActivatedNodeIDs []string
}

func NewState(fleetID string, nodeID string) (State, error) {
	normalizedNodeID, errorValue := NormalizeNodeID(nodeID)
	if errorValue != nil {
		return State{}, errorValue
	}
	return State{
		FleetID: strings.TrimSpace(fleetID),
		Members: []Member{{
			NodeID: normalizedNodeID,
			Status: MemberStatusActive,
		}},
	}, nil
}

func NormalizeNodeID(nodeID string) (string, error) {
	normalizedNodeID := strings.Trim(strings.ToLower(strings.TrimSpace(nodeID)), "-")
	if normalizedNodeID == "" {
		return "", errors.New("node id is required")
	}
	if !isNodeIDCharacter(normalizedNodeID[0]) || !isNodeIDCharacter(normalizedNodeID[len(normalizedNodeID)-1]) {
		return "", errors.New("node id must start and end with a letter or number")
	}
	for _, character := range normalizedNodeID {
		if isNodeIDCharacter(byte(character)) || character == '-' {
			continue
		}
		return "", errors.New("node id may contain only lowercase letters, numbers, and hyphens")
	}
	return normalizedNodeID, nil
}

func (state State) AddNode(nodeID string) (JoinResult, error) {
	normalizedNodeID, errorValue := NormalizeNodeID(nodeID)
	if errorValue != nil {
		return JoinResult{}, errorValue
	}
	if member, ok := state.FindMember(normalizedNodeID); ok {
		return JoinResult{State: state, Member: member}, nil
	}
	nextState := state.copy()
	nextState.Members = append(nextState.Members, Member{
		NodeID: normalizedNodeID,
		Status: MemberStatusPending,
	})
	activatedNodeIDs := nextState.promotePendingNodes()
	member, _ := nextState.FindMember(normalizedNodeID)
	return JoinResult{
		State:            nextState,
		Member:           member,
		ActivatedNodeIDs: activatedNodeIDs,
	}, nil
}

func (state State) RemoveNode(nodeID string) (State, error) {
	normalizedNodeID, errorValue := NormalizeNodeID(nodeID)
	if errorValue != nil {
		return State{}, errorValue
	}
	nextState := State{FleetID: state.FleetID}
	for _, member := range state.Members {
		if member.NodeID != normalizedNodeID {
			nextState.Members = append(nextState.Members, member)
			continue
		}
		if member.Status == MemberStatusActive {
			return State{}, errors.New("active node removal would create an even voting fleet; add a replacement or reset to standalone")
		}
	}
	return nextState, nil
}

func (state State) ResetStandalone(nodeID string) (State, error) {
	return NewState(state.FleetID, nodeID)
}

func (state State) FindMember(nodeID string) (Member, bool) {
	for _, member := range state.Members {
		if member.NodeID == nodeID {
			return member, true
		}
	}
	return Member{}, false
}

func (state State) ActiveMembers() []Member {
	return state.membersByStatus(MemberStatusActive)
}

func (state State) PendingMembers() []Member {
	return state.membersByStatus(MemberStatusPending)
}

func (state State) QuorumSize() int {
	activeCount := len(state.ActiveMembers())
	if activeCount == 0 {
		return 0
	}
	return activeCount/2 + 1
}

func (state State) CanCommit(availableNodeIDs []string) bool {
	if len(availableNodeIDs) == 0 {
		return false
	}
	availableNodes := map[string]bool{}
	for _, nodeID := range availableNodeIDs {
		normalizedNodeID, errorValue := NormalizeNodeID(nodeID)
		if errorValue == nil {
			availableNodes[normalizedNodeID] = true
		}
	}
	availableActiveCount := 0
	for _, member := range state.ActiveMembers() {
		if availableNodes[member.NodeID] {
			availableActiveCount++
		}
	}
	return availableActiveCount >= state.QuorumSize()
}

func (state State) SelectOwner(key string) (Member, bool) {
	activeMembers := state.ActiveMembers()
	if len(activeMembers) == 0 {
		return Member{}, false
	}
	sort.Slice(activeMembers, func(leftIndex int, rightIndex int) bool {
		return activeMembers[leftIndex].NodeID < activeMembers[rightIndex].NodeID
	})
	selectedMember := activeMembers[0]
	selectedScore := rendezvousScore(key, selectedMember.NodeID)
	for _, member := range activeMembers[1:] {
		score := rendezvousScore(key, member.NodeID)
		if score <= selectedScore {
			continue
		}
		selectedMember = member
		selectedScore = score
	}
	return selectedMember, true
}

func (state State) copy() State {
	members := append([]Member(nil), state.Members...)
	return State{FleetID: state.FleetID, Members: members}
}

func (state *State) promotePendingNodes() []string {
	if len(state.ActiveMembers()) == 0 {
		activatedNodeID := state.activateOldestPendingNode()
		if activatedNodeID == "" {
			return nil
		}
		return []string{activatedNodeID}
	}

	activatedNodeIDs := []string{}
	for len(state.PendingMembers()) >= 2 {
		firstNodeID := state.activateOldestPendingNode()
		secondNodeID := state.activateOldestPendingNode()
		if firstNodeID == "" || secondNodeID == "" {
			return activatedNodeIDs
		}
		activatedNodeIDs = append(activatedNodeIDs, firstNodeID, secondNodeID)
	}
	return activatedNodeIDs
}

func (state *State) activateOldestPendingNode() string {
	for index, member := range state.Members {
		if member.Status != MemberStatusPending {
			continue
		}
		state.Members[index].Status = MemberStatusActive
		return member.NodeID
	}
	return ""
}

func (state State) membersByStatus(status string) []Member {
	members := []Member{}
	for _, member := range state.Members {
		if member.Status == status {
			members = append(members, member)
		}
	}
	return members
}

func isNodeIDCharacter(character byte) bool {
	return character >= 'a' && character <= 'z' || character >= '0' && character <= '9'
}

func rendezvousScore(key string, nodeID string) uint64 {
	digest := sha256.Sum256([]byte(key + "\x00" + nodeID))
	return binary.BigEndian.Uint64(digest[:8])
}
