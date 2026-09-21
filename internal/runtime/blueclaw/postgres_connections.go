package blueclaw

import "time"

const (
	PostgresMaxConnections               = 100
	PostgresSuperuserReservedConnections = 3

	MessengerDatabaseConnections = 50
	AgentDatabaseConnections     = 30
	AdminDatabaseConnections     = 12
	OperatorDatabaseConnections  = 5

	DatabaseConnectionMaxIdleTime = 10 * time.Minute
	DatabaseConnectionMaxLifetime = 30 * time.Minute
)

func PostgresConnectionsGranted() int {
	return MessengerDatabaseConnections + AgentDatabaseConnections + AdminDatabaseConnections + OperatorDatabaseConnections
}

func PostgresConnectionsAvailable() int {
	return PostgresMaxConnections - PostgresSuperuserReservedConnections
}
