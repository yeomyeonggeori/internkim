package packagerepository

import (
	"fmt"
	"slices"
	"strings"
)

// A channel is how far a build is trusted, and it is the one thing the apt,
// rpm and pacman repositories are all split by: `stable` is what an install
// follows and `testing` carries release candidates. apt calls it a suite.
const (
	StableChannel  = "stable"
	TestingChannel = "testing"
)

var Channels = []string{StableChannel, TestingChannel}

const DefaultChannel = StableChannel

func CheckChannel(channel string) error {
	if slices.Contains(Channels, channel) {
		return nil
	}
	return fmt.Errorf("channel %q is not one of %s", channel, strings.Join(Channels, ", "))
}

// Package is one package file about to be indexed.
type Package struct {
	FileName string
	Contents []byte
}

// Signer is what a repository needs from the archive key. One key signs every
// format, and each format asks for the form it reads.
type Signer interface {
	ClearSign(document []byte) ([]byte, error)
	DetachSign(document []byte) ([]byte, error)
	DetachSignBinary(document []byte) ([]byte, error)
	PublicKeyring() ([]byte, error)
	PublicKeyArmoured() ([]byte, error)
}

var _ Signer = (*GPGSigner)(nil)
