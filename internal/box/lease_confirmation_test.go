package box

import (
	"testing"
	"time"
)

func TestARestoreConfirmedByThePlaneLeavesTheDaemonALeaseThatStillRuns(t *testing.T) {
	_, daemon, clock, _ := boxTakingOver(t)
	*clock = clock.Add(80 * time.Second)

	if errorValue := ConfirmTheLease(daemon.Places.StateDirectoryPath, *clock); errorValue != nil {
		t.Fatal(errorValue)
	}

	lease, isKept := daemon.serviceLease()
	if !isKept || !lease.runsOutAt().After(*clock) || lease.LapseSeconds != 90 {
		t.Fatalf("after the plane confirmed the box mid-restore the lease is %+v (kept %v), running out at %s", lease, isKept, lease.runsOutAt())
	}
}

func TestConfirmingWithoutALeaseKeepsNone(t *testing.T) {
	directory := t.TempDir()

	if errorValue := ConfirmTheLease(directory, time.Now()); errorValue != nil {
		t.Fatal(errorValue)
	}

	if _, isKept := (Daemon{Places: Places{StateDirectoryPath: directory}}).serviceLease(); isKept {
		t.Fatal("confirming a box that keeps no lease made it keep one")
	}
}
