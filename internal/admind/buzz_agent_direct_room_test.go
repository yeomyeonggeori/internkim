package admind

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/buzzidentity"
)

const (
	roomWithTheAgent         = "00000000-0000-0000-0000-0000000000d1"
	deletedRoomWithTheAgent  = "00000000-0000-0000-0000-0000000000d2"
	roomWithAColleague       = "00000000-0000-0000-0000-0000000000d3"
	groupRoomWithTheAgent    = "00000000-0000-0000-0000-0000000000d4"
	roomThePersonLeft        = "00000000-0000-0000-0000-0000000000d5"
	streamRoomWithTheAgent   = "00000000-0000-0000-0000-0000000000d6"
	firstRoomTheAgentOpened  = "00000000-0000-0000-0000-0000000000e1"
	secondRoomTheAgentOpened = "00000000-0000-0000-0000-0000000000e2"
)

func TestOnlyALiveTwoPersonRoomWithTheAgentCountsAsTheAgentDirectRoom(t *testing.T) {
	relay := disposableRelayDatabase(t)
	if _, errorValue := relay.Exec(roomShapeCreateStatement); errorValue != nil {
		t.Fatalf("create the room tables: %v", errorValue)
	}
	agentKey := derivedKey(t, buzzidentity.AgentSubject)
	holdingTheRoom := derivedKey(t, versionedSubject("holding@example.com", 1))
	whoseRoomWasDeleted := derivedKey(t, versionedSubject("deleted@example.com", 1))
	talkingToAColleague := derivedKey(t, versionedSubject("colleague@example.com", 1))
	colleague := derivedKey(t, versionedSubject("other@example.com", 1))
	inAGroupRoom := derivedKey(t, versionedSubject("group@example.com", 1))
	whoLeftTheRoom := derivedKey(t, versionedSubject("left@example.com", 1))
	inAStreamRoom := derivedKey(t, versionedSubject("stream@example.com", 1))
	withNoRoomAtAll := derivedKey(t, versionedSubject("nobody@example.com", 1))

	holdRoom(t, relay, roomWithTheAgent, "dm", "private", "", "")
	seatInRoom(t, relay, roomWithTheAgent, holdingTheRoom, false)
	seatInRoom(t, relay, roomWithTheAgent, agentKey, false)
	holdRoom(t, relay, deletedRoomWithTheAgent, "dm", "private", "", "now()")
	seatInRoom(t, relay, deletedRoomWithTheAgent, whoseRoomWasDeleted, false)
	seatInRoom(t, relay, deletedRoomWithTheAgent, agentKey, false)
	holdRoom(t, relay, roomWithAColleague, "dm", "private", "", "")
	seatInRoom(t, relay, roomWithAColleague, talkingToAColleague, false)
	seatInRoom(t, relay, roomWithAColleague, colleague, false)
	holdRoom(t, relay, groupRoomWithTheAgent, "dm", "private", "", "")
	seatInRoom(t, relay, groupRoomWithTheAgent, inAGroupRoom, false)
	seatInRoom(t, relay, groupRoomWithTheAgent, colleague, false)
	seatInRoom(t, relay, groupRoomWithTheAgent, agentKey, false)
	holdRoom(t, relay, roomThePersonLeft, "dm", "private", "", "")
	seatInRoom(t, relay, roomThePersonLeft, whoLeftTheRoom, true)
	seatInRoom(t, relay, roomThePersonLeft, agentKey, false)
	holdRoom(t, relay, streamRoomWithTheAgent, "stream", "private", "", "")
	seatInRoom(t, relay, streamRoomWithTheAgent, inAStreamRoom, false)
	seatInRoom(t, relay, streamRoomWithTheAgent, agentKey, false)

	missing, errorValue := membersWithoutTheAgentDirectRoom(context.Background(), relay, agentKey, []string{
		holdingTheRoom, whoseRoomWasDeleted, talkingToAColleague, inAGroupRoom, whoLeftTheRoom, inAStreamRoom, withNoRoomAtAll,
	})
	if errorValue != nil {
		t.Fatalf("read who has no room with the agent: %v", errorValue)
	}

	sort.Strings(missing)
	expected := []string{whoseRoomWasDeleted, talkingToAColleague, inAGroupRoom, whoLeftTheRoom, inAStreamRoom, withNoRoomAtAll}
	sort.Strings(expected)
	if !reflect.DeepEqual(missing, expected) {
		t.Fatalf("only a live room holding just the person and the agent is their room with it: got %v, want %v", missing, expected)
	}
}

func TestEveryAdmittedMemberGetsOneRoomWithTheAgentAndASecondPassOpensNothing(t *testing.T) {
	relay := disposableRelayDatabase(t)
	if _, errorValue := relay.Exec(roomShapeCreateStatement); errorValue != nil {
		t.Fatalf("create the room tables: %v", errorValue)
	}
	agentKey := derivedKey(t, buzzidentity.AgentSubject)
	firstKey := derivedKey(t, versionedSubject("first@example.com", 1))
	secondKey := derivedKey(t, versionedSubject("second@example.com", 1))
	outsiderKey := derivedKey(t, versionedSubject("outsider@example.com", 1))
	holdRelayMembers(t, relay, map[string]string{agentKey: "owner", firstKey: "member", secondKey: "member"})

	var ensuredFor []string
	roomsToOpen := []string{firstRoomTheAgentOpened, secondRoomTheAgentOpened}
	service := serviceHoldingTheSeed(t)
	service.Configuration.ChatdEndpoint = "http://chatd.test"
	service.Configuration.ChatdPlatform = "buzz"
	service.Configuration.BuzzDatabaseURL = "postgres://the-disposable-relay"
	service.buzzDatabaseOwner.database = relay
	service.buzzDatabaseOwner.connectionURL = service.Configuration.BuzzDatabaseURL
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if isBlueclawPolicyGet(request) {
			return jsonResponse(http.StatusOK, `{"people":[
				{"personID":"p-1","emails":["first@example.com"]},
				{"personID":"p-2","emails":["second@example.com"]},
				{"personID":"p-3","emails":["outsider@example.com"]}]}`, nil), nil
		}
		if request.URL.String() != "http://chatd.test/v1/platform/buzz/dm.ensure" {
			return nil, errors.New("nothing else answers in this test")
		}
		pubkey := openedRoomPubkey(t, request.Body)
		if len(roomsToOpen) == 0 {
			t.Fatalf("a room was asked for after every member already had one: %s", pubkey)
		}
		channelID := roomsToOpen[0]
		roomsToOpen = roomsToOpen[1:]
		holdRoom(t, relay, channelID, "dm", "private", "", "")
		seatInRoom(t, relay, channelID, pubkey, false)
		seatInRoom(t, relay, channelID, agentKey, false)
		ensuredFor = append(ensuredFor, pubkey)
		return jsonResponse(http.StatusOK, `{"channelID":"`+channelID+`"}`, nil), nil
	})}

	members := service.memberBuzzMembers(context.Background())
	service.openTheAgentDirectRoomForEveryMember(context.Background(), members)
	service.openTheAgentDirectRoomForEveryMember(context.Background(), members)

	sort.Strings(ensuredFor)
	expected := []string{firstKey, secondKey}
	sort.Strings(expected)
	if !reflect.DeepEqual(ensuredFor, expected) {
		t.Fatalf("each admitted member gets one room with the agent, the outsider %s none yet: got %v, want %v", outsiderKey, ensuredFor, expected)
	}
	if waiting := service.admittedMembersWithoutTheAgentDirectRoom(context.Background(), members); len(waiting) != 0 {
		t.Fatalf("nobody admitted may still be waiting for a room with the agent: %v", waiting)
	}
}

func openedRoomPubkey(t *testing.T, body io.Reader) string {
	t.Helper()
	var ensureRequest struct {
		UserSecretHex        string `json:"userSecretHex"`
		ChannelID            string `json:"channelId"`
		CounterpartPubkeyHex string `json:"counterpartPubkeyHex"`
	}
	if errorValue := json.NewDecoder(body).Decode(&ensureRequest); errorValue != nil {
		t.Fatalf("decode dm.ensure: %v", errorValue)
	}
	if ensureRequest.ChannelID != "" || ensureRequest.CounterpartPubkeyHex != "" {
		t.Fatalf("the room with the agent is asked for with no channel and no counterpart: %+v", ensureRequest)
	}
	pubkey, errorValue := buzzPublicKey(ensureRequest.UserSecretHex)
	if errorValue != nil {
		t.Fatalf("dm.ensure carried no usable secret: %v", errorValue)
	}
	return strings.ToLower(pubkey)
}

func TestNoRoomIsAskedForWhileTheRelayCannotSayWhoItAdmits(t *testing.T) {
	service := serviceHoldingTheSeed(t)
	service.Configuration.ChatdEndpoint = "http://chatd.test"
	service.Configuration.ChatdPlatform = "buzz"
	service.Configuration.BuzzDatabaseURL = "postgres://buzz@127.0.0.1:1/buzz?sslmode=disable"
	service.HTTPClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if isBlueclawPolicyGet(request) {
			return jsonResponse(http.StatusOK, `{"people":[{"personID":"p-1","emails":["first@example.com"]}]}`, nil), nil
		}
		if strings.HasPrefix(request.URL.String(), "http://chatd.test/") {
			t.Fatalf("a room was asked for although the relay could not say who it admits: %s", request.URL.String())
		}
		return nil, errors.New("nothing else answers in this test")
	})}

	service.openTheAgentDirectRoomForEveryMember(context.Background(), service.memberBuzzMembers(context.Background()))
}
