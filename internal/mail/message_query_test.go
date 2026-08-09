package mail

import (
	"testing"

	"github.com/emersion/go-imap/v2"
)

func TestMailMessagePageUIDsReturnNewestFirst(t *testing.T) {
	uids := MessagePageUIDs([]imap.UID{7, 9, 8}, 2)
	if len(uids) != 2 || uids[0] != 9 || uids[1] != 8 {
		t.Fatalf("uids = %#v", uids)
	}
}
