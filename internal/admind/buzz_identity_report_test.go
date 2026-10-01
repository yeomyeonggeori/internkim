package admind

import (
	"testing"

	nostr "github.com/nbd-wtf/go-nostr"

	"github.com/yeomyeonggeori/internkim/internal/buzzidentity"
)

func testReportPubkey(t *testing.T, seed string, subject string) string {
	t.Helper()
	pubkey, errorValue := nostr.GetPublicKey(buzzidentity.Secret(seed, subject))
	if errorValue != nil {
		t.Fatalf("derive pubkey: %v", errorValue)
	}
	return pubkey
}

func TestBuzzIdentityReportLaysOldAndCurrentKeysSideBySide(t *testing.T) {
	seed := "1a" + "00000000000000000000000000000000000000000000000000000000000000"
	emails := []string{"sample@example.com"}
	versionOf := func(string) int { return 2 }
	owners, errorValue := buzzKeyOwners(seed, emails, versionOf)
	if errorValue != nil {
		t.Fatalf("owners: %v", errorValue)
	}

	oldPubkey := testReportPubkey(t, seed, versionedSubject("sample@example.com", 1))
	currentPubkey := testReportPubkey(t, seed, versionedSubject("sample@example.com", 2))
	bootstrapPubkey := testReportPubkey(t, seed, buzzidentity.BootstrapSubject)

	rows := buzzIdentityLedgerRows{
		profileByPubkey: map[string]buzzProfileContent{
			oldPubkey: {Name: "이샘플", Picture: "http://127.0.0.1:3000/media/abc"},
		},
		membershipByPubkey: map[string]int{oldPubkey: 3, bootstrapPubkey: 1},
		messagesByPubkey:   map[string]int{currentPubkey: 12, oldPubkey: 5},
		channels: []buzzLedgerChannel{
			{id: "dm-1", channelType: "dm", members: []string{oldPubkey, bootstrapPubkey}},
			{id: "stream-1", name: "잡담", channelType: "stream", members: []string{currentPubkey}},
		},
	}

	report, errorValue := assembleBuzzIdentityReport(emails, versionOf, seed, owners, rows)
	if errorValue != nil {
		t.Fatalf("assemble: %v", errorValue)
	}

	if len(report.People) != 3 || len(report.People[2].Identities) != 2 {
		t.Fatalf("expected bootstrap, agent, and one person with two identities: %+v", report.People)
	}
	old := report.People[2].Identities[0]
	current := report.People[2].Identities[1]
	if old.IsCurrent || !current.IsCurrent {
		t.Fatalf("version 2 must be the current identity: %+v %+v", old, current)
	}
	if !old.HasProfile || !old.HasPicture || old.Memberships != 3 {
		t.Fatalf("the old key holds the profile and the rooms: %+v", old)
	}
	if current.HasProfile || current.Messages != 12 {
		t.Fatalf("the current key has no profile yet carries the messages: %+v", current)
	}

	if len(report.Channels) != 1 {
		t.Fatalf("only the channel holding a stale member is reported: %+v", report.Channels)
	}
	dm := report.Channels[0]
	if dm.ChannelID != "dm-1" {
		t.Fatalf("the dm with the old key is the stale one: %+v", dm)
	}
	if dm.Members[0].Owner != "sample@example.com" || dm.Members[0].IsCurrent {
		t.Fatalf("the old key is owned but not current: %+v", dm.Members[0])
	}
	if dm.Members[1].Owner != "bootstrap" {
		t.Fatalf("the bootstrap key is named: %+v", dm.Members[1])
	}
}

func TestBuzzIdentityReportNamesTheKeyNobodyDerives(t *testing.T) {
	seed := "2b" + "00000000000000000000000000000000000000000000000000000000000000"
	owners, errorValue := buzzKeyOwners(seed, nil, func(string) int { return 1 })
	if errorValue != nil {
		t.Fatalf("owners: %v", errorValue)
	}
	rows := buzzIdentityLedgerRows{
		profileByPubkey:    map[string]buzzProfileContent{},
		membershipByPubkey: map[string]int{},
		messagesByPubkey:   map[string]int{},
		channels: []buzzLedgerChannel{
			{id: "dm-2", channelType: "dm", members: []string{"feedfacefeedfacefeedfacefeedfacefeedfacefeedfacefeedfacefeedface"}},
		},
	}
	report, errorValue := assembleBuzzIdentityReport(nil, func(string) int { return 1 }, seed, owners, rows)
	if errorValue != nil {
		t.Fatalf("assemble: %v", errorValue)
	}
	if len(report.Channels) != 1 || report.Channels[0].Members[0].Owner != "" {
		t.Fatalf("a stranger key carries no owner: %+v", report.Channels)
	}
}
