package companyhost

import (
	"errors"
	"io"
	"slices"
	"testing"

	"github.com/yeomyeonggeori/internkim/internal/runtime/blueclaw"
)

func TestABoxStopsForAMoveItsMessengerLastOfItsWritersThenTheDatabaseAndOnlyThenTheFollowers(t *testing.T) {
	machine := &recordedMachine{}
	stopsBeforeTheBarrier := -1
	keep := func() error {
		stopsBeforeTheBarrier = len(machine.runs)
		return nil
	}

	if errorValue := stopForTheMove(linuxPlatform{}, machine, io.Discard, keep); errorValue != nil {
		t.Fatal(errorValue)
	}

	if len(machine.runs) != 8 {
		t.Fatalf("ran %v", machine.runs)
	}
	for index, run := range machine.runs {
		if want := []string{"disable", "stop"}[index%2]; run[1] != want {
			t.Fatalf("run %d was %v; each step keeps its services from starting again, then stops them", index, run)
		}
	}
	agent, messenger, database, followers := machine.runs[1], machine.runs[3], machine.runs[5], machine.runs[7]
	relay := blueclaw.RelayServiceName + ".service"
	wal, files := blueclaw.CompanyHostWALFollowServiceName+".service", blueclaw.CompanyHostFileFollowServiceName+".service"
	if !slices.Contains(agent, blueclaw.BlueclawServiceName+".service") || slices.Contains(agent, relay) || slices.Contains(agent, wal) || slices.Contains(agent, files) {
		t.Fatalf("the first stop was %v; it stops the agent and everything else that writes while members can still send", agent)
	}
	if !slices.Contains(messenger, relay) || slices.Contains(messenger, blueclaw.BlueclawServiceName+".service") {
		t.Fatalf("the second stop was %v; the messenger stops on its own, so members wait only from there", messenger)
	}
	if !slices.Contains(database, blueclaw.CompanyHostDatabaseServiceName+".service") {
		t.Fatalf("the third stop was %v; the database stops while the followers still keep what it sends", database)
	}
	if !slices.Contains(followers, wal) || !slices.Contains(followers, files) {
		t.Fatalf("the last stop was %v", followers)
	}
	if stopsBeforeTheBarrier != 4 {
		t.Fatalf("the file barrier was asked after %d stops; it is asked once every writer stopped and before the database does", stopsBeforeTheBarrier)
	}
}

func TestABoxWhoseLastFilesWereNotKeptLeavesItsDatabaseRunning(t *testing.T) {
	machine := &recordedMachine{}

	errorValue := stopForTheMove(linuxPlatform{}, machine, io.Discard, func() error { return errors.New("the file journal did not answer") })

	if errorValue == nil || len(machine.runs) != 4 {
		t.Fatalf("got %v after %v; a move whose last file changes were not kept stops nothing more", errorValue, machine.runs)
	}
}
