package blueclaw

import "time"

const (
	PostgresMaxConnections               = 100
	PostgresSuperuserReservedConnections = 3

	MessengerDatabaseConnections = 50
	AgentDatabaseConnections     = 30
	AdminDatabaseConnections     = 12

	DatabaseConnectionMaxIdleTime = 10 * time.Minute
	DatabaseConnectionMaxLifetime = 30 * time.Minute

	// The share travels to blueclaw as configuration because it is a separate
	// Go module under a different licence, so this name is spelled once here,
	// once in blueclaw's own struct tag, and nowhere else on this side.
	// TestTheAgentShareTravelsUnderTheNameBlueclawReads holds the two together.
	AgentDatabaseConnectionsField = "maxOpenConnections"
)

func PostgresConnectionsGranted() int {
	return MessengerDatabaseConnections + AgentDatabaseConnections + AdminDatabaseConnections
}

func PostgresConnectionsAvailable() int {
	return PostgresMaxConnections - PostgresSuperuserReservedConnections
}
