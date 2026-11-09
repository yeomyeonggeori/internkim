package walstream

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"
)

const (
	StandbyName         = "internkim_standby"
	duplicateObjectCode = "42710"
	drainWait           = 5 * time.Millisecond
	idleWait            = 10 * time.Second
	largestPieceBytes   = 8 * 1024 * 1024
)

type Keeper interface {
	Keep(ctx context.Context, piece Piece) error
	NewestEnd(ctx context.Context, timeline int32) (pglogrepl.LSN, bool, error)
	KeepHistory(ctx context.Context, history TimelineHistory) error
}

type Follower struct {
	ReplicationConnection string
	Kept                  Keeper
	OnStreaming           func(ctx context.Context) error
}

func (follower Follower) Run(ctx context.Context) error {
	connection, errorValue := pgconn.Connect(ctx, follower.ReplicationConnection)
	if errorValue != nil {
		return fmt.Errorf("connecting to the company database for replication: %w", errorValue)
	}
	defer connection.Close(context.Background())
	system, errorValue := pglogrepl.IdentifySystem(ctx, connection)
	if errorValue != nil {
		return fmt.Errorf("identifying the company database: %w", errorValue)
	}
	start, errorValue := follower.startingPoint(ctx, connection, system)
	if errorValue != nil {
		return errorValue
	}
	if errorValue := reserveTheSlot(ctx, connection); errorValue != nil {
		return errorValue
	}
	if errorValue := pglogrepl.StartReplication(ctx, connection, StandbyName, start, pglogrepl.StartReplicationOptions{Timeline: system.Timeline, Mode: pglogrepl.PhysicalReplication}); errorValue != nil {
		return fmt.Errorf("streaming the company database's WAL from %s on timeline %d: %w", start, system.Timeline, errorValue)
	}
	log.Printf("following the company database's WAL from %s on timeline %d", start, system.Timeline)
	if follower.OnStreaming != nil {
		if errorValue := follower.OnStreaming(ctx); errorValue != nil {
			return errorValue
		}
	}
	stream := walStream{connection: connection, kept: follower.Kept, pending: Piece{Timeline: system.Timeline, Start: start}}
	return stream.follow(ctx)
}

func (follower Follower) startingPoint(ctx context.Context, connection *pgconn.PgConn, system pglogrepl.IdentifySystemResult) (pglogrepl.LSN, error) {
	newestEnd, isFound, errorValue := follower.Kept.NewestEnd(ctx, system.Timeline)
	if errorValue != nil {
		return 0, fmt.Errorf("reading where the company's kept WAL ends: %w", errorValue)
	}
	if !isFound {
		return follower.firstPositionOn(ctx, connection, system)
	}
	if newestEnd > system.XLogPos {
		return 0, fmt.Errorf("the company's kept WAL runs to %s on timeline %d, past this database's %s; this database is older than what the company keeps, so it must take over from the kept WAL instead of serving", newestEnd, system.Timeline, system.XLogPos)
	}
	return newestEnd, nil
}

func (follower Follower) firstPositionOn(ctx context.Context, connection *pgconn.PgConn, system pglogrepl.IdentifySystemResult) (pglogrepl.LSN, error) {
	if system.Timeline == 1 {
		return segmentStartOf(system.XLogPos), nil
	}
	answered, errorValue := pglogrepl.TimelineHistory(ctx, connection, system.Timeline)
	if errorValue != nil {
		return 0, fmt.Errorf("reading the history of timeline %d: %w", system.Timeline, errorValue)
	}
	history := TimelineHistory{Timeline: system.Timeline, Content: answered.Content}
	began, errorValue := history.Began()
	if errorValue != nil {
		return 0, errorValue
	}
	if errorValue := follower.Kept.KeepHistory(ctx, history); errorValue != nil {
		return 0, errorValue
	}
	return segmentStartOf(began), nil
}

func reserveTheSlot(ctx context.Context, connection *pgconn.PgConn) error {
	_, errorValue := pglogrepl.CreateReplicationSlot(ctx, connection, StandbyName, "", pglogrepl.CreateReplicationSlotOptions{Mode: pglogrepl.PhysicalReplication, SnapshotAction: "RESERVE_WAL"})
	var databaseError *pgconn.PgError
	if errors.As(errorValue, &databaseError) && databaseError.Code == duplicateObjectCode {
		return nil
	}
	if errorValue != nil {
		return fmt.Errorf("reserving the replication slot %s: %w", StandbyName, errorValue)
	}
	return nil
}

type walStream struct {
	connection *pgconn.PgConn
	kept       Keeper
	pending    Piece
	lastReport time.Time
}

func (stream *walStream) follow(ctx context.Context) error {
	for {
		if errorValue := stream.keepWhenLarge(ctx); errorValue != nil {
			return errorValue
		}
		wait := idleWait
		if len(stream.pending.Bytes) > 0 {
			wait = drainWait
		}
		receiving, cancel := context.WithTimeout(ctx, wait)
		message, errorValue := stream.connection.ReceiveMessage(receiving)
		cancel()
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if pgconn.Timeout(errorValue) {
			if len(stream.pending.Bytes) > 0 {
				if errorValue := stream.keep(ctx); errorValue != nil {
					return errorValue
				}
			}
			if errorValue := stream.reportWhenDue(ctx); errorValue != nil {
				return errorValue
			}
			continue
		}
		if errorValue != nil {
			return fmt.Errorf("receiving the company database's WAL: %w", errorValue)
		}
		if errorValue := stream.take(ctx, message); errorValue != nil {
			return errorValue
		}
	}
}

func (stream *walStream) keepWhenLarge(ctx context.Context) error {
	if len(stream.pending.Bytes) < largestPieceBytes {
		return nil
	}
	return stream.keep(ctx)
}

func (stream *walStream) take(ctx context.Context, message pgproto3.BackendMessage) error {
	switch message := message.(type) {
	case *pgproto3.CopyData:
		return stream.takeCopyData(ctx, message.Data)
	case *pgproto3.ErrorResponse:
		return fmt.Errorf("the company database stopped streaming its WAL: %w", pgconn.ErrorResponseToPgError(message))
	case *pgproto3.CopyDone:
		return fmt.Errorf("the company database ended the WAL stream at %s", stream.pending.End())
	}
	return nil
}

func (stream *walStream) takeCopyData(ctx context.Context, data []byte) error {
	if len(data) == 0 {
		return nil
	}
	switch data[0] {
	case pglogrepl.PrimaryKeepaliveMessageByteID:
		keepalive, errorValue := pglogrepl.ParsePrimaryKeepaliveMessage(data[1:])
		if errorValue != nil {
			return fmt.Errorf("reading the company database's keepalive: %w", errorValue)
		}
		if keepalive.ReplyRequested {
			return stream.report(ctx)
		}
	case pglogrepl.XLogDataByteID:
		record, errorValue := pglogrepl.ParseXLogData(data[1:])
		if errorValue != nil {
			return fmt.Errorf("reading the company database's WAL: %w", errorValue)
		}
		if record.WALStart != stream.pending.End() {
			return fmt.Errorf("the company database sent WAL from %s while this box had it to %s; a gap would make the kept WAL unusable", record.WALStart, stream.pending.End())
		}
		stream.pending.Bytes = append(stream.pending.Bytes, record.WALData...)
	}
	return nil
}

func (stream *walStream) keep(ctx context.Context) error {
	if errorValue := stream.kept.Keep(ctx, stream.pending); errorValue != nil {
		return errorValue
	}
	stream.pending = Piece{Timeline: stream.pending.Timeline, Start: stream.pending.End()}
	return stream.report(ctx)
}

func (stream *walStream) reportWhenDue(ctx context.Context) error {
	if time.Since(stream.lastReport) < idleWait {
		return nil
	}
	return stream.report(ctx)
}

func (stream *walStream) report(ctx context.Context) error {
	kept := stream.pending.Start
	if errorValue := pglogrepl.SendStandbyStatusUpdate(ctx, stream.connection, pglogrepl.StandbyStatusUpdate{WALWritePosition: kept, WALFlushPosition: kept, WALApplyPosition: kept}); errorValue != nil {
		return fmt.Errorf("telling the company database the WAL to %s is kept: %w", kept, errorValue)
	}
	stream.lastReport = time.Now()
	return nil
}

func segmentStartOf(position pglogrepl.LSN) pglogrepl.LSN {
	return position - position%walSegmentBytes
}

const walSegmentBytes = 16 * 1024 * 1024
