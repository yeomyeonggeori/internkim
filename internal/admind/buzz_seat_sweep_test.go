package admind

import (
	"reflect"
	"testing"
)

func TestSeatsNobodyAccountsForNamesOnlyTheKeysNoAddressDerives(t *testing.T) {
	held := map[string]string{
		"aa": "owner",
		"bb": "member",
		"cc": "member",
	}
	accounted := map[string]bool{"aa": true, "cc": true}
	if actual := seatsNobodyAccountsFor(held, accounted); !reflect.DeepEqual(actual, []string{"bb"}) {
		t.Errorf("seatsNobodyAccountsFor = %v, want [bb]", actual)
	}
}

func TestSeatsNobodyAccountsForKeepsAWholeRoomItAccountsFor(t *testing.T) {
	held := map[string]string{"aa": "owner", "bb": "member"}
	accounted := map[string]bool{"aa": true, "bb": true}
	if actual := seatsNobodyAccountsFor(held, accounted); len(actual) != 0 {
		t.Errorf("seatsNobodyAccountsFor = %v, want none", actual)
	}
}
