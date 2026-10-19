package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/box"
	"github.com/yeomyeonggeori/internkim/internal/companyhost"
	"github.com/yeomyeonggeori/internkim/internal/hostupdate"
	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

const (
	standbyReleaseWait     = 20 * time.Minute
	standbyReleaseInterval = 15 * time.Second
)

func thisBoxRelease() string {
	return hostupdate.TagOf(companyhost.PackageVersion)
}

func followTheServiceBoxRelease(targetVersion string) error {
	supervisor := hostupdate.LocalSupervisor()
	if supervisor.IsRunning() {
		return nil
	}
	hostCommandPath := blueclaw.CompanyPackageBinaryRoot + "/" + blueclaw.CompanyPackageName
	return supervisor.Start(hostCommandPath, targetVersion)
}

func standbyBoxesTakeItFirst(targetVersion string) func() error {
	return func() error {
		connected, errorValue := box.LoadConnected(blueclaw.CompanyHostBoxStatePath)
		if errorValue != nil {
			return nil
		}
		connection, errorValue := companyhost.ReadConnection(companyhost.CurrentConnectionPath())
		if errorValue != nil {
			return fmt.Errorf("reading the company's connection: %w", errorValue)
		}
		client := box.Client{AppURL: connection.AppURL}
		deadline := time.Now().Add(standbyReleaseWait)
		for {
			shortOfIt, errorValue := client.AimStandbyBoxesAt(context.Background(), connected.Keyring.Identity, targetVersion)
			if errorValue != nil {
				return errorValue
			}
			if len(shortOfIt) == 0 {
				return nil
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("the standby boxes %s did not take %s within %s, so this box stays where it is", namesOf(shortOfIt), targetVersion, standbyReleaseWait)
			}
			log.Printf("waiting for the standby boxes %s to take %s before this box does", namesOf(shortOfIt), targetVersion)
			time.Sleep(standbyReleaseInterval)
		}
	}
}

func namesOf(boxes []box.BoxShortOfRelease) string {
	names := make([]string, 0, len(boxes))
	for _, short := range boxes {
		name := short.HostName
		if name == "" {
			name = short.PublicKey
		}
		names = append(names, name)
	}
	return strings.Join(names, ", ")
}
