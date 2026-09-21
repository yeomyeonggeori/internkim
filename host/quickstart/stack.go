package quickstart

import (
	_ "embed"
	"strings"
)

//go:embed compose.yaml
var ComposeFile []byte

//go:embed 01-buzz.sql
var MessengerDatabaseFile []byte

//go:embed buzz-image
var messengerImage string

func MessengerImage() string {
	return strings.TrimSpace(messengerImage)
}
