package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/nbd-wtf/go-nostr"
	"gitlab.com/eastriver/internkim/internal/buzzidentity"
)

type hostIdentity struct {
	AgentPrivateKey     string `json:"agentPrivateKey"`
	RelayOwnerPublicKey string `json:"relayOwnerPublicKey"`
}

func main() {
	seedPath := flag.String("seed", "", "file containing the company's Buzz identity seed")
	outputPath := flag.String("output", "", "private JSON identity file to write")
	flag.Parse()
	if errorValue := writeHostIdentity(*seedPath, *outputPath); errorValue != nil {
		fmt.Fprintln(os.Stderr, errorValue)
		os.Exit(1)
	}
}

func writeHostIdentity(seedPath, outputPath string) error {
	seed, errorValue := os.ReadFile(seedPath)
	if errorValue != nil {
		return fmt.Errorf("read identity seed: %w", errorValue)
	}
	identity, errorValue := identityForSeed(strings.TrimSpace(string(seed)))
	if errorValue != nil {
		return errorValue
	}
	document, errorValue := json.Marshal(identity)
	if errorValue != nil {
		return errorValue
	}
	return writePrivateIdentity(outputPath, document)
}

func identityForSeed(seed string) (hostIdentity, error) {
	if seed == "" {
		return hostIdentity{}, fmt.Errorf("the identity seed file is empty")
	}
	ownerPublicKey, errorValue := nostr.GetPublicKey(buzzidentity.Secret(seed, buzzidentity.BootstrapSubject))
	if errorValue != nil {
		return hostIdentity{}, errorValue
	}
	return hostIdentity{
		AgentPrivateKey:     buzzidentity.Secret(seed, buzzidentity.AgentSubject),
		RelayOwnerPublicKey: ownerPublicKey,
	}, nil
}

func writePrivateIdentity(outputPath string, document []byte) error {
	file, errorValue := os.OpenFile(outputPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if errorValue != nil {
		return errorValue
	}
	defer file.Close()
	if errorValue := file.Chmod(0600); errorValue != nil {
		return errorValue
	}
	_, errorValue = file.Write(document)
	return errorValue
}
