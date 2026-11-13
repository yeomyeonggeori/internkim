//go:build linux

package filestream

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestADatabaseAtRestStopsBeingKeptAfterItIsRead(t *testing.T) {
	kept := &keptObjects{objects: map[string][]byte{}}
	follower := aFollower(t, kept)
	writeADatabase(t, filepath.Join(follower.Roots[0].Path, "tasks.db"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ended := make(chan error, 1)
	go func() { ended <- follower.Run(ctx) }()

	time.Sleep(6 * time.Second)
	settled := kept.journalEntries()
	time.Sleep(3 * time.Second)
	later := kept.journalEntries()

	if settled == 0 {
		t.Fatal("the follower kept nothing, so the test watched nothing")
	}
	if later != settled {
		t.Fatalf("the journal grew from %d to %d entries while nothing changed: keeping the database is seen as a change to it", settled, later)
	}
	cancel()
	<-ended
}

func writeADatabase(t *testing.T, databasePath string) {
	t.Helper()
	database, errorValue := sql.Open("sqlite", "file:"+databasePath+"?_pragma=journal_mode(WAL)")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer database.Close()
	if _, errorValue := database.Exec("create table task (id integer primary key, title text); insert into task (title) values ('first')"); errorValue != nil {
		t.Fatal(errorValue)
	}
}
