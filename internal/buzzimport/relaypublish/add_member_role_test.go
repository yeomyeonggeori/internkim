package relaypublish

import "testing"

func TestAnAddThatNamesNoRoleCarriesNoRoleTag(t *testing.T) {
	tags := addMemberTags("channel-1", "ABCDEF", "")

	if len(tags) != 2 {
		t.Fatalf("tags = %v, want only the channel and the person", tags)
	}
	if tags[1][1] != "abcdef" {
		t.Fatalf("pubkey tag = %q, want it lowercased", tags[1][1])
	}
}

func TestAnAddThatNamesARoleCarriesIt(t *testing.T) {
	tags := addMemberTags("channel-1", "abcdef", " owner ")

	if len(tags) != 3 {
		t.Fatalf("tags = %v, want the role alongside the channel and the person", tags)
	}
	if tags[2][0] != "role" || tags[2][1] != "owner" {
		t.Fatalf("role tag = %v, want [role owner]", tags[2])
	}
}
