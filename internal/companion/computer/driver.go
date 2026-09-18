package computer

import (
	"context"
	"encoding/json"
)

// Driver is one open Cua Driver session: every call carries the same session
// label, and the refs and captures it hands back belong to that session alone.
type Driver interface {
	Call(ctx context.Context, tool string, arguments map[string]any) (json.RawMessage, error)
	Close() error
}

type DriverOpener interface {
	Open(ctx context.Context, sessionLabel string) (Driver, error)
}
