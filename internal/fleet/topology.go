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
	BoardID string
	Status  string
}

type State struct {
	FleetID string
	Members []Member
}

type JoinResult struct {
	State             State
	Member            Member
	ActivatedBoardIDs []string
}

func NewState(fleetID string, boardID string) (State, error) {
	normalizedBoardID, errorValue := NormalizeBoardID(boardID)
	if errorValue != nil {
		return State{}, errorValue
	}
	return State{
		FleetID: strings.TrimSpace(fleetID),
		Members: []Member{{
			BoardID: normalizedBoardID,
			Status:  MemberStatusActive,
		}},
	}, nil
}

func NormalizeBoardID(boardID string) (string, error) {
	normalizedBoardID := strings.Trim(strings.ToLower(strings.TrimSpace(boardID)), "-")
	if normalizedBoardID == "" {
		return "", errors.New("board id is required")
	}
	if !isBoardIDCharacter(normalizedBoardID[0]) || !isBoardIDCharacter(normalizedBoardID[len(normalizedBoardID)-1]) {
		return "", errors.New("board id must start and end with a letter or number")
	}
	for _, character := range normalizedBoardID {
		if isBoardIDCharacter(byte(character)) || character == '-' {
			continue
		}
		return "", errors.New("board id may contain only lowercase letters, numbers, and hyphens")
	}
	return normalizedBoardID, nil
}

func (state State) AddBoard(boardID string) (JoinResult, error) {
	normalizedBoardID, errorValue := NormalizeBoardID(boardID)
	if errorValue != nil {
		return JoinResult{}, errorValue
	}
	if member, ok := state.FindMember(normalizedBoardID); ok {
		return JoinResult{State: state, Member: member}, nil
	}
	nextState := state.copy()
	nextState.Members = append(nextState.Members, Member{
		BoardID: normalizedBoardID,
		Status:  MemberStatusPending,
	})
	activatedBoardIDs := nextState.promotePendingBoards()
	member, _ := nextState.FindMember(normalizedBoardID)
	return JoinResult{
		State:             nextState,
		Member:            member,
		ActivatedBoardIDs: activatedBoardIDs,
	}, nil
}

func (state State) RemoveBoard(boardID string) (State, error) {
	normalizedBoardID, errorValue := NormalizeBoardID(boardID)
	if errorValue != nil {
		return State{}, errorValue
	}
	nextState := State{FleetID: state.FleetID}
	for _, member := range state.Members {
		if member.BoardID != normalizedBoardID {
			nextState.Members = append(nextState.Members, member)
			continue
		}
		if member.Status == MemberStatusActive {
			return State{}, errors.New("active board removal would create an even voting fleet; add a replacement or reset to standalone")
		}
	}
	return nextState, nil
}

func (state State) ResetStandalone(boardID string) (State, error) {
	return NewState(state.FleetID, boardID)
}

func (state State) FindMember(boardID string) (Member, bool) {
	for _, member := range state.Members {
		if member.BoardID == boardID {
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

func (state State) CanCommit(availableBoardIDs []string) bool {
	if len(availableBoardIDs) == 0 {
		return false
	}
	availableBoards := map[string]bool{}
	for _, boardID := range availableBoardIDs {
		normalizedBoardID, errorValue := NormalizeBoardID(boardID)
		if errorValue == nil {
			availableBoards[normalizedBoardID] = true
		}
	}
	availableActiveCount := 0
	for _, member := range state.ActiveMembers() {
		if availableBoards[member.BoardID] {
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
		return activeMembers[leftIndex].BoardID < activeMembers[rightIndex].BoardID
	})
	selectedMember := activeMembers[0]
	selectedScore := rendezvousScore(key, selectedMember.BoardID)
	for _, member := range activeMembers[1:] {
		score := rendezvousScore(key, member.BoardID)
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

func (state *State) promotePendingBoards() []string {
	if len(state.ActiveMembers()) == 0 {
		activatedBoardID := state.activateOldestPendingBoard()
		if activatedBoardID == "" {
			return nil
		}
		return []string{activatedBoardID}
	}

	activatedBoardIDs := []string{}
	for len(state.PendingMembers()) >= 2 {
		firstBoardID := state.activateOldestPendingBoard()
		secondBoardID := state.activateOldestPendingBoard()
		if firstBoardID == "" || secondBoardID == "" {
			return activatedBoardIDs
		}
		activatedBoardIDs = append(activatedBoardIDs, firstBoardID, secondBoardID)
	}
	return activatedBoardIDs
}

func (state *State) activateOldestPendingBoard() string {
	for index, member := range state.Members {
		if member.Status != MemberStatusPending {
			continue
		}
		state.Members[index].Status = MemberStatusActive
		return member.BoardID
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

func isBoardIDCharacter(character byte) bool {
	return character >= 'a' && character <= 'z' || character >= '0' && character <= '9'
}

func rendezvousScore(key string, boardID string) uint64 {
	digest := sha256.Sum256([]byte(key + "\x00" + boardID))
	return binary.BigEndian.Uint64(digest[:8])
}
