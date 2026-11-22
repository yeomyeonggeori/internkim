package box

import (
	"context"
	"testing"
)

func servingBoxAskedToMove(t *testing.T) (*fakePlane, Daemon, *serviceSwitches, *[]Session) {
	t.Helper()
	plane := &fakePlane{isClaimed: true, accessToken: "service.session.token", keepsBackups: true, takeOver: TakeOverPolicy{IsAutomatic: true, LapseSeconds: 90}, moveOutcome: MoveStopped}
	recorded := &installs{}
	daemon, places := daemonFor(t, plane, recorded)
	plane.sealedModelKey = sealTo(t, identityOf(t, places), "sk-or-v1-sample")
	keepInstalledCredentials(t, places)
	runSteps(t, daemon, 1)
	if installedCompanyIn(places.StateDirectoryPath) != sampleCompanyID {
		t.Fatal("the service box did not install")
	}
	switches := &serviceSwitches{}
	daemon.StopServices = func() error { switches.downs++; return nil }
	daemon.StandDown = func() error { switches.forgotten++; return nil }
	daemon.StandUp = func() error { switches.ups++; return nil }
	handedOver := []Session{}
	daemon.HandOver = func(_ context.Context, session Session) (LogPosition, error) {
		handedOver = append(handedOver, session)
		entry := int64(41)
		return LogPosition{WALTimeline: 2, WALPosition: "0/3000148", FileEntry: &entry}, nil
	}
	plane.move = &Move{FromBox: identityOf(t, places).PublicKey(), ToBox: freshTestIdentity(t).PublicKey()}
	return plane, daemon, switches, &handedOver
}

func TestAServiceBoxAskedToMoveStopsAndReportsWhereTheCompanysLogsEnd(t *testing.T) {
	plane, daemon, switches, handedOver := servingBoxAskedToMove(t)

	runSteps(t, daemon, 1)

	if len(*handedOver) != 1 {
		t.Fatalf("a service box asked to move handed over %d times", len(*handedOver))
	}
	if len(plane.movesReported) != 1 || plane.movesReported[0].WALPosition != "0/3000148" || *plane.movesReported[0].FileEntry != 41 {
		t.Fatalf("reported %+v", plane.movesReported)
	}
	if !daemon.hasStoodDown() || switches.ups != 0 || switches.forgotten != 0 {
		t.Fatalf("stood down %v, started %d times, let the company go %d times", daemon.hasStoodDown(), switches.ups, switches.forgotten)
	}
}

func TestABoxThatReportedWhereItStoppedWaitsForTheMoveWithItsServicesStopped(t *testing.T) {
	plane, daemon, switches, handedOver := servingBoxAskedToMove(t)
	runSteps(t, daemon, 1)
	plane.move.Stop = &plane.movesReported[0]

	waits := waitsOverSteps(t, daemon, 3)

	if len(*handedOver) != 1 || len(plane.movesReported) != 1 || switches.ups != 0 {
		t.Fatalf("handed over %d times, reported %d times, started %d times", len(*handedOver), len(plane.movesReported), switches.ups)
	}
	for _, wait := range waits {
		if wait != moveCheckInterval {
			t.Fatalf("a box in a move waited %s between checks", wait)
		}
	}
}

func TestAMoveCalledOffStartsTheServicesItStopped(t *testing.T) {
	plane, daemon, switches, _ := servingBoxAskedToMove(t)
	runSteps(t, daemon, 1)

	plane.move = nil
	runSteps(t, daemon, 1)

	if switches.ups != 1 || daemon.hasStoodDown() {
		t.Fatalf("a box whose move was called off started its services %d times; still stood down: %v", switches.ups, daemon.hasStoodDown())
	}
}

func TestABoxAMoveIsAskedToTakesOverOnlyOnceTheOldBoxHasStopped(t *testing.T) {
	plane := &fakePlane{isClaimed: true, accessToken: "standby.session.token", role: RoleStandby, keepsBackups: true, takeOver: TakeOverPolicy{IsAutomatic: true, LapseSeconds: 90}}
	daemon, places := daemonFor(t, plane, &installs{})
	plane.sealedModelKey = sealTo(t, identityOf(t, places), "sk-or-v1-sample")
	plane.move = &Move{FromBox: freshTestIdentity(t).PublicKey(), ToBox: identityOf(t, places).PublicKey()}
	tookOver, stoodDown := 0, 0
	isUnderway := false
	daemon.TakeOver = func(context.Context, Session, Keyring) error {
		tookOver++
		isUnderway = true
		return ErrTakeOverUnderway
	}
	daemon.TakeOverIsUnderway = func() bool { return isUnderway }
	daemon.StandDown = func() error { stoodDown++; return nil }

	waits := waitsOverSteps(t, daemon, 2)
	if tookOver != 0 {
		t.Fatalf("a box took over %d times before the box it moves from stopped", tookOver)
	}
	if waits[0] != moveCheckInterval {
		t.Fatalf("a box waiting for a move checked again after %s", waits[0])
	}

	entry := int64(41)
	plane.move.Stop = &LogPosition{WALTimeline: 2, WALPosition: "0/3000148", FileEntry: &entry}
	runSteps(t, daemon, 2)

	if tookOver != 2 || stoodDown != 0 {
		t.Fatalf("once the old box stopped, a standby the company moves to took over %d times and stood down %d times", tookOver, stoodDown)
	}
}
