package admind

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"strings"
	"sync"
	"time"

	blueclawruntime "gitlab.com/eastriver/internkim/internal/runtime/blueclaw"
)

type buzzDatabaseHandle struct {
	mutex         sync.Mutex
	database      *sql.DB
	connectionURL string
	waitCount     int64
}

func (service *Service) buzzDatabase() (*sql.DB, error) {
	connectionURL := strings.TrimSpace(service.Configuration.BuzzDatabaseURL)
	if connectionURL == "" {
		return nil, errors.New("the buzz database url is not configured")
	}
	handle := &service.buzzDatabaseOwner
	handle.mutex.Lock()
	defer handle.mutex.Unlock()
	if handle.database != nil && handle.connectionURL == connectionURL {
		reportBuzzDatabaseWaiting(handle)
		return handle.database, nil
	}
	database, errorValue := sql.Open("postgres", connectionURL)
	if errorValue != nil {
		return nil, errorValue
	}
	boundToTheConnectionBudget(database, blueclawruntime.AdminDatabaseConnections)
	if handle.database != nil {
		_ = handle.database.Close()
	}
	handle.database = database
	handle.connectionURL = connectionURL
	handle.waitCount = 0
	return database, nil
}

// database/sql has no acquire timeout: a caller that finds the share full waits
// for a free connection until its own context ends, and a caller carrying
// context.Background() waits for the life of the process. The messenger's pool
// bounds the wait alone and leaves the query unbounded (buzz-db's DbConfig, at
// three seconds); Go offers no such split, so the only deadline available bounds
// the queries too and has to be long enough for the work rather than for the
// wait. These two are: what one person's action sets off, and a pass over
// everything, which its own ticker loop already runs one at a time.
const (
	buzzDatabaseRequestBudget = 2 * time.Minute
	buzzDatabaseSweepBudget   = 30 * time.Minute
)

// Work a person's action sets off runs after the answer has gone, so nothing
// waits on it and nothing notices it never ending. A sign-in burst started one
// of these per sign-in.
func inTheBackgroundWithin(budget time.Duration, work func(ctx context.Context)) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), budget)
		defer cancel()
		work(ctx)
	}()
}

func boundToTheConnectionBudget(database *sql.DB, connections int) {
	database.SetMaxOpenConns(connections)
	database.SetMaxIdleConns(connections)
	database.SetConnMaxIdleTime(blueclawruntime.DatabaseConnectionMaxIdleTime)
	database.SetConnMaxLifetime(blueclawruntime.DatabaseConnectionMaxLifetime)
}

func reportBuzzDatabaseWaiting(handle *buzzDatabaseHandle) {
	statistics := handle.database.Stats()
	if statistics.WaitCount <= handle.waitCount {
		return
	}
	waitsSinceLastReport := statistics.WaitCount - handle.waitCount
	handle.waitCount = statistics.WaitCount
	log.Printf(
		"admind ran out of its buzz database budget: %d callers queued for a connection (%s waited in total) against the %d admind is granted, so the database is answering and admind is rationing itself",
		waitsSinceLastReport,
		statistics.WaitDuration.Round(time.Millisecond),
		statistics.MaxOpenConnections,
	)
}

// A sweep is best-effort work on a clock: it runs again on the next tick, so it
// is better for one to give up than for one queued for a connection to hold the
// clock for the life of the process. Its own loop runs one at a time, so the
// budget bounds the pathological pass rather than spacing them out.
func (service *Service) withinASweepBudget(ctx context.Context, sweep func(ctx context.Context)) {
	sweepContext, cancel := context.WithTimeout(ctx, buzzDatabaseSweepBudget)
	defer cancel()
	sweep(sweepContext)
}
