package box

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/workqueue"
)

func TestAStandbyBoxHoldsTheCompanyKeyInstallsNothingAndKeepsUpWithTheServiceBox(t *testing.T) {
	plane := &fakePlane{isClaimed: true, accessToken: "standby.session.token", role: RoleStandby}
	recorded := &installs{}
	daemon, places := daemonFor(t, plane, recorded)
	plane.sealedModelKey = sealTo(t, identityOf(t, places), "sk-or-v1-sample")
	keptUp := 0
	daemon.StartKeepingUp = func(_ context.Context, session Session) {
		if session.Session.AccessToken != "standby.session.token" {
			t.Fatalf("kept up with %+v", session.Session)
		}
		keptUp++
	}

	runSteps(t, daemon, 2)

	if len(recorded.requests) != 0 {
		t.Fatalf("a standby box installed the company's services: %+v", recorded.requests)
	}
	if keptUp != 2 {
		t.Fatalf("a standby box kept up with the service box %d times in two checks", keptUp)
	}
}

func TestABoxWaitsForTheCompanyKeyAndThenServesWithIt(t *testing.T) {
	holder := freshTestIdentity(t)
	plane := &fakePlane{isClaimed: true, accessToken: "added.session.token", sealingKey: holder.EncryptionPublicKey()}
	plane.sealedModelKey = sealTo(t, holder, "sk-or-v1-sample")
	recorded := &installs{}
	daemon, places := daemonFor(t, plane, recorded)

	runSteps(t, daemon, 1)
	if len(recorded.requests) != 0 {
		t.Fatal("a box without the company key installed before it could open the model key")
	}

	own := identityOf(t, places)
	handed, errorValue := SealSecretTo(own.EncryptionPublicKey(), holder.encryptionKey.Bytes(), CompanyKeyPurpose(sampleCompanyID, own.EncryptionPublicKey()))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	plane.sealedCompanyKey = &handed
	runSteps(t, daemon, 1)

	if len(recorded.requests) != 1 || recorded.requests[0].ModelKey != "sk-or-v1-sample" {
		t.Fatalf("installs = %+v", recorded.requests)
	}
	if _, errorValue := os.Stat(companyKeyPathIn(places.StateDirectoryPath)); errorValue != nil {
		t.Fatalf("the box kept no company key: %v", errorValue)
	}
}

func TestACompanyKeyThatIsNotTheOneTheCompanySealsToIsRefused(t *testing.T) {
	holder := freshTestIdentity(t)
	stranger := freshTestIdentity(t)
	plane := &fakePlane{isClaimed: true, accessToken: "added.session.token", sealingKey: holder.EncryptionPublicKey()}
	recorded := &installs{}
	daemon, places := daemonFor(t, plane, recorded)
	own := identityOf(t, places)

	handed, errorValue := SealSecretTo(own.EncryptionPublicKey(), stranger.encryptionKey.Bytes(), CompanyKeyPurpose(sampleCompanyID, own.EncryptionPublicKey()))
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, errorValue := KeyringFor(own, places.StateDirectoryPath, sampleCompanyID, holder.EncryptionPublicKey(), &handed); errorValue == nil {
		t.Fatal("a box kept a company key that is not the key the company seals to")
	}
	if _, errorValue := os.Stat(companyKeyPathIn(places.StateDirectoryPath)); !os.IsNotExist(errorValue) {
		t.Fatal("a refused company key was written down")
	}
	plane.sealedCompanyKey = &handed
	runSteps(t, daemon, 1)
	if len(recorded.requests) != 0 {
		t.Fatal("a box installed with a company key it should have refused")
	}
}

func TestAServiceBoxHandsTheCompanyKeyToTheBoxesWaitingForIt(t *testing.T) {
	waiter := freshTestIdentity(t)
	plane := &fakePlane{isClaimed: true, accessToken: "service.session.token"}
	plane.awaiting = []AwaitingBox{{PublicKey: waiter.PublicKey(), EncryptionKey: waiter.EncryptionPublicKey()}}
	recorded := &installs{}
	daemon, places := daemonFor(t, plane, recorded)
	own := identityOf(t, places)

	runSteps(t, daemon, 1)

	if len(plane.handedOn) != 1 || len(plane.handedOn[0]) != 1 || plane.handedOn[0][0].PublicKey != waiter.PublicKey() {
		t.Fatalf("handed on %+v", plane.handedOn)
	}
	keyring, errorValue := KeyringFor(waiter, t.TempDir(), sampleCompanyID, own.EncryptionPublicKey(), &plane.handedOn[0][0].Sealed)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !keyring.HoldsCompanyKey(own.EncryptionPublicKey()) {
		t.Fatal("the waiting box could not take the key it was handed")
	}
}

func TestABoxWhoseOwnKeyIsTheSealingKeyKeepsItAsTheCompanyKey(t *testing.T) {
	own := freshTestIdentity(t)
	directory := t.TempDir()
	if errorValue := storeCompanyKey(directory, sampleCompanyID, freshTestIdentity(t).encryptionKey); errorValue != nil {
		t.Fatal(errorValue)
	}

	keyring, errorValue := KeyringFor(own, directory, sampleCompanyID, own.EncryptionPublicKey(), nil)

	if errorValue != nil || !keyring.HoldsCompanyKey(own.EncryptionPublicKey()) {
		t.Fatalf("keyring %+v, %v", keyring, errorValue)
	}
	loaded, errorValue := LoadKeyring(own, directory, sampleCompanyID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if sealingKey, isHeld := loaded.SealingKey(); !isHeld || sealingKey != own.EncryptionPublicKey() {
		t.Fatalf("offline, the box seals to %q (held %v), not its own key", sealingKey, isHeld)
	}
}

func TestABoxWaitingForTheCompanyKeyHasNothingToSealBackupsTo(t *testing.T) {
	waiter := freshTestIdentity(t)

	keyring, errorValue := LoadKeyring(waiter, t.TempDir(), sampleCompanyID)

	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if _, isHeld := keyring.SealingKey(); isHeld {
		t.Fatal("a box that was handed nothing claimed a sealing key")
	}
	if _, errorValue := keyring.SealCompanyKeyFor(sampleCompanyID, []AwaitingBox{{PublicKey: "other", EncryptionKey: waiter.EncryptionPublicKey()}}); !errors.Is(errorValue, ErrCompanyKeyNotHeld) {
		t.Fatalf("a box without the company key handed one on: %v", errorValue)
	}
}

func TestABoxMadeTheServiceBoxTakesOverFromTheNewestBackupInsteadOfStartingFresh(t *testing.T) {
	plane := &fakePlane{isClaimed: true, accessToken: "promoted.session.token"}
	recorded := &installs{}
	daemon, places := daemonFor(t, plane, recorded)
	plane.sealedModelKey = sealTo(t, identityOf(t, places), "sk-or-v1-sample")
	tookOver := 0
	daemon.TakeOver = func(_ context.Context, session Session, keyring Keyring) error {
		tookOver++
		if session.Configuration.Company.ID != sampleCompanyID || !keyring.HoldsCompanyKey(session.SealingKey) {
			t.Fatalf("took over with %+v", session)
		}
		return ErrTakeOverUnderway
	}

	runSteps(t, daemon, 2)

	if tookOver != 2 || len(recorded.requests) != 0 {
		t.Fatalf("took over %d times and installed %+v", tookOver, recorded.requests)
	}
}

func TestABoxTakesOverFromABackupWithoutTheModelKeyAFreshInstallNeeds(t *testing.T) {
	plane := &fakePlane{isClaimed: true, accessToken: "promoted.session.token"}
	recorded := &installs{}
	daemon, _ := daemonFor(t, plane, recorded)
	tookOver := 0
	daemon.TakeOver = func(context.Context, Session, Keyring) error {
		tookOver++
		return ErrTakeOverUnderway
	}

	runSteps(t, daemon, 1)

	if tookOver != 1 || len(recorded.requests) != 0 {
		t.Fatalf("took over %d times and installed %+v", tookOver, recorded.requests)
	}
}

func TestABoxWithNoBackupToTakeOverFromStartsTheCompanyFresh(t *testing.T) {
	plane := &fakePlane{isClaimed: true, accessToken: "first.session.token"}
	recorded := &installs{}
	daemon, places := daemonFor(t, plane, recorded)
	plane.sealedModelKey = sealTo(t, identityOf(t, places), "sk-or-v1-sample")
	daemon.TakeOver = func(context.Context, Session, Keyring) error { return ErrNothingToTakeOver }

	runSteps(t, daemon, 1)

	if len(recorded.requests) != 1 {
		t.Fatalf("installs = %+v", recorded.requests)
	}
}

func TestABoxThatCannotOpenTheBackupsTheCompanyKeepsNeverStartsItFresh(t *testing.T) {
	plane := &fakePlane{isClaimed: true, accessToken: "promoted.session.token", keepsBackups: true}
	recorded := &installs{}
	daemon, places := daemonFor(t, plane, recorded)
	plane.sealedModelKey = sealTo(t, identityOf(t, places), "sk-or-v1-sample")
	daemon.TakeOver = func(context.Context, Session, Keyring) error { return ErrNothingToTakeOver }

	runSteps(t, daemon, 2)

	if len(recorded.requests) != 0 {
		t.Fatal("a box that could not open the company's backups started the company empty")
	}
}

func TestATakeOverThatFailedIsNotStartedAgainBehindTheAdministratorsBack(t *testing.T) {
	plane := &fakePlane{isClaimed: true, accessToken: "promoted.session.token"}
	recorded := &installs{}
	daemon, places := daemonFor(t, plane, recorded)
	plane.sealedModelKey = sealTo(t, identityOf(t, places), "sk-or-v1-sample")
	daemon.TakeOver = func(context.Context, Session, Keyring) error { return errors.New("the last take-over failed") }

	runSteps(t, daemon, 1)

	if len(recorded.requests) != 0 {
		t.Fatal("a box whose take-over failed started the company empty")
	}
}

func TestAServiceBoxMadeAStandbyStopsItsServicesAndTakesOverAgainWhenItServesOnceMore(t *testing.T) {
	plane := &fakePlane{isClaimed: true, accessToken: "service.session.token"}
	recorded := &installs{}
	daemon, places := daemonFor(t, plane, recorded)
	plane.sealedModelKey = sealTo(t, identityOf(t, places), "sk-or-v1-sample")
	runSteps(t, daemon, 1)
	if installedCompanyIn(places.StateDirectoryPath) != sampleCompanyID {
		t.Fatal("the service box did not install")
	}
	stoodDown := 0
	daemon.StandDown = func() error { stoodDown++; return nil }
	tookOver := 0
	daemon.TakeOver = func(context.Context, Session, Keyring) error { tookOver++; return ErrTakeOverUnderway }

	plane.role = RoleStandby
	runSteps(t, daemon, 2)
	if stoodDown != 1 || installedCompanyIn(places.StateDirectoryPath) != "" {
		t.Fatalf("stood down %d times; installed %q", stoodDown, installedCompanyIn(places.StateDirectoryPath))
	}

	plane.role = RoleService
	runSteps(t, daemon, 1)
	if tookOver != 1 || len(recorded.requests) != 1 {
		t.Fatalf("serving again took over %d times and installed %d times", tookOver, len(recorded.requests))
	}
}

func TestAServiceBoxItsCompanyLetsGoStopsItsServicesBeforeAnnouncing(t *testing.T) {
	plane := &fakePlane{isClaimed: true, accessToken: "service.session.token"}
	recorded := &installs{}
	daemon, places := daemonFor(t, plane, recorded)
	plane.sealedModelKey = sealTo(t, identityOf(t, places), "sk-or-v1-sample")
	runSteps(t, daemon, 1)
	if installedCompanyIn(places.StateDirectoryPath) != sampleCompanyID {
		t.Fatal("the service box did not install")
	}
	stoodDown := 0
	daemon.StandDown = func() error { stoodDown++; return nil }

	plane.isClaimed = false
	runSteps(t, daemon, 2)
	if stoodDown != 1 || installedCompanyIn(places.StateDirectoryPath) != "" {
		t.Fatalf("stood down %d times; installed %q", stoodDown, installedCompanyIn(places.StateDirectoryPath))
	}
}

type serviceSwitches struct{ downs, ups int }

func cutOffServiceBox(t *testing.T, takeOver TakeOverPolicy) (*fakePlane, *installs, Daemon, *time.Time, *serviceSwitches) {
	t.Helper()
	plane := &fakePlane{isClaimed: true, accessToken: "service.session.token", takeOver: takeOver}
	recorded := &installs{}
	daemon, places := daemonFor(t, plane, recorded)
	plane.sealedModelKey = sealTo(t, identityOf(t, places), "sk-or-v1-sample")
	clock := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	switches := &serviceSwitches{}
	daemon.Now = func() time.Time { return clock }
	daemon.StandDown = func() error { switches.downs++; return nil }
	daemon.StandUp = func() error { switches.ups++; return nil }
	runSteps(t, daemon, 1)
	if len(recorded.requests) != 1 {
		t.Fatalf("installs = %+v", recorded.requests)
	}
	plane.isAway = true
	return plane, recorded, daemon, &clock, switches
}

func TestAServiceBoxCutOffForHalfTheLapseStopsItsServicesAndStartsThemWhenConfirmedAgain(t *testing.T) {
	plane, recorded, daemon, clock, switches := cutOffServiceBox(t, TakeOverPolicy{IsAutomatic: true, LapseSeconds: 300})
	*clock = clock.Add(100 * time.Second)
	runSteps(t, daemon, 1)
	if switches.downs != 0 {
		t.Fatal("the box stopped its services before half the lapse passed")
	}

	*clock = clock.Add(51 * time.Second)
	runSteps(t, daemon, 2)
	if switches.downs != 1 || installedCompanyIn(daemon.Places.StateDirectoryPath) != sampleCompanyID {
		t.Fatalf("stood down %d times, installed %q", switches.downs, installedCompanyIn(daemon.Places.StateDirectoryPath))
	}

	plane.isAway = false
	runSteps(t, daemon, 2)
	if switches.ups != 1 || len(recorded.requests) != 1 {
		t.Fatalf("stood up %d times and installed %d times", switches.ups, len(recorded.requests))
	}
}

func TestAServiceBoxThatNoStandbyTakesOverFromKeepsServingWhileCutOff(t *testing.T) {
	_, _, daemon, clock, switches := cutOffServiceBox(t, TakeOverPolicy{IsAutomatic: false, LapseSeconds: 300})
	*clock = clock.Add(24 * time.Hour)
	runSteps(t, daemon, 2)

	if switches.downs != 0 {
		t.Fatal("a box no standby takes over from stopped serving while it was cut off")
	}
}

func TestOnlyTheServiceBoxKeepsTheCompanyQueueAddress(t *testing.T) {
	plane := &fakePlane{isClaimed: true, accessToken: "service.session.token"}
	recorded := &installs{}
	daemon, places := daemonFor(t, plane, recorded)
	daemon.Places.WorkQueuePath = filepath.Join(t.TempDir(), "work-queue.json")
	plane.sealedModelKey = sealTo(t, identityOf(t, places), "sk-or-v1-sample")

	runSteps(t, daemon, 1)

	address, isRead := workqueue.Read(daemon.Places.WorkQueuePath)
	if !isRead || address.CompanyID != sampleCompanyID || address.GatewayURL != "wss://gateway.example.test" {
		t.Fatalf("wrote %+v (%v)", address, isRead)
	}

	plane.role = RoleStandby
	runSteps(t, daemon, 1)
	if _, isRead := workqueue.Read(daemon.Places.WorkQueuePath); isRead {
		t.Fatal("a box that stood by kept the queue address, so it would still put work in the queue")
	}
}

func TestAStandbyBoxTakesWorkAndTakesTheReleaseItIsAimedAt(t *testing.T) {
	plane := &fakePlane{isClaimed: true, accessToken: "standby.session.token", role: RoleStandby, targetVersion: "v0.0.2"}
	recorded := &installs{}
	daemon, places := daemonFor(t, plane, recorded)
	plane.sealedModelKey = sealTo(t, identityOf(t, places), "sk-or-v1-sample")
	daemon.PackageVersion = "v0.0.1"
	working, followed := 0, []string{}
	daemon.StartWorking = func(context.Context, Session) { working++ }
	daemon.FollowRelease = func(targetVersion string) error { followed = append(followed, targetVersion); return nil }

	runSteps(t, daemon, 1)
	daemon.PackageVersion = "v0.0.2"
	runSteps(t, daemon, 1)

	if working != 2 || len(followed) != 1 || followed[0] != "v0.0.2" {
		t.Fatalf("worked %d times and took %v", working, followed)
	}
}
