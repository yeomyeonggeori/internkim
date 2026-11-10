package main

import (
	"context"
	"fmt"
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
		order := box.ReleaseOrder{
			Client:         box.Client{AppURL: connection.AppURL},
			Identity:       connected.Keyring.Identity,
			PackageVersion: thisBoxRelease(),
			Wait:           standbyReleaseWait,
			Interval:       standbyReleaseInterval,
			Sleep:          time.Sleep,
		}
		return order.StandbyBoxesTakeItFirst(context.Background(), targetVersion)
	}
}
