package admind

import (
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
