package main

import (
	"testing"

	"gitlab.com/eastriver/internkim/internal/buzzimport"
)

func TestOnlyAnOpenRoomImportsAsAnOpenChannel(t *testing.T) {
	for _, testCase := range []struct {
		name        string
		channelType string
		want        string
	}{
		{name: "an open room anyone on the team can already join", channelType: buzzimport.OpenChannelType, want: "open"},
		{name: "a private room, which is what a circle channel is", channelType: "P", want: "private"},
		{name: "a direct conversation", channelType: buzzimport.DirectChannelType, want: "private"},
		{name: "a group conversation", channelType: buzzimport.GroupChannelType, want: "private"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			visibility := importedChannelVisibility(buzzimport.MattermostChannel{Type: testCase.channelType})
			if visibility != testCase.want {
				t.Fatalf("visibility = %q; want %q. An open channel is one anybody in the company walks into with kind:9021, invitation or not", visibility, testCase.want)
			}
		})
	}
}
