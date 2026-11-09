package walstream

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
)

func RequireTheStandby(ctx context.Context, databaseConnection string) error {
	return changeSettings(ctx, databaseConnection,
		"ALTER SYSTEM RESET restore_command",
		"ALTER SYSTEM RESET recovery_target_timeline",
		"ALTER SYSTEM SET synchronous_standby_names = '"+StandbyName+"'",
	)
}

func ReleaseTheStandby(ctx context.Context, databaseConnection string) error {
	return changeSettings(ctx, databaseConnection,
		"ALTER SYSTEM RESET synchronous_standby_names",
		"SELECT pg_drop_replication_slot(slot_name) FROM pg_replication_slots WHERE slot_name = '"+StandbyName+"'",
	)
}

func changeSettings(ctx context.Context, databaseConnection string, statements ...string) error {
	connection, errorValue := pgconn.Connect(ctx, databaseConnection)
	if errorValue != nil {
		return fmt.Errorf("connecting to the company database to change its settings: %w", errorValue)
	}
	defer connection.Close(context.Background())
	for _, each := range append(statements, "SELECT pg_reload_conf()") {
		if _, errorValue := connection.Exec(ctx, each).ReadAll(); errorValue != nil {
			return fmt.Errorf("running %q on the company database: %w", each, errorValue)
		}
	}
	return nil
}
