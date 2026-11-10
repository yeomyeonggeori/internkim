package box

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
)

type ReleaseOrder struct {
	Client         Client
	Identity       Identity
	PackageVersion string
	Wait           time.Duration
	Interval       time.Duration
	Sleep          func(time.Duration)
}

func (order ReleaseOrder) StandbyBoxesTakeItFirst(ctx context.Context, targetVersion string) error {
	session, isKnown, errorValue := order.Client.Session(ctx, order.Identity, order.PackageVersion)
	if errorValue != nil {
		return fmt.Errorf("asking whether this box serves its company before it takes %s: %w", targetVersion, errorValue)
	}
	if isKnown && session.Role == RoleStandby {
		return nil
	}
	deadline := order.Client.now().Add(order.Wait)
	for {
		shortOfIt, errorValue := order.Client.AimStandbyBoxesAt(ctx, order.Identity, targetVersion)
		if errorValue != nil {
			return errorValue
		}
		if len(shortOfIt) == 0 {
			return nil
		}
		if order.Client.now().After(deadline) {
			return fmt.Errorf("the standby boxes %s did not take %s within %s, so this box stays where it is", namesOf(shortOfIt), targetVersion, order.Wait)
		}
		log.Printf("waiting for the standby boxes %s to take %s before this box does", namesOf(shortOfIt), targetVersion)
		order.Sleep(order.Interval)
	}
}

func namesOf(boxes []BoxShortOfRelease) string {
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
