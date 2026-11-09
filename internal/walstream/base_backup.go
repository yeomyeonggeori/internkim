package walstream

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/jackc/pglogrepl"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"

	"github.com/yeomyeonggeori/internkim/internal/box"
	"github.com/yeomyeonggeori/internkim/internal/hostbackup"
	"github.com/yeomyeonggeori/internkim/internal/offsite"
)

const (
	baseBackupVersion      = 1
	baseBackupFolder       = "base"
	baseBackupChunkFolder  = "base-chunks"
	baseBackupNamePrefix   = "internkim-base-"
	baseBackupNameSuffix   = ".json"
	baseBackupTimeLayout   = "20060102T150405Z"
	DataDirectoryArchive   = "base.tar"
	BackupManifestArchive  = "backup_manifest"
	newArchiveMessage      = 'n'
	archiveDataMessage     = 'd'
	manifestMessage        = 'm'
	progressMessage        = 'p'
	oldestStreamingVersion = 15
)

var ErrNoBaseBackup = errors.New("the company keeps no base backup this box can open")

type BaseBackup struct {
	Version    int                 `json:"version"`
	Name       string              `json:"name"`
	SealingKey string              `json:"sealingKey"`
	CreatedAt  time.Time           `json:"createdAt"`
	SystemID   string              `json:"systemID"`
	Timeline   int32               `json:"timeline"`
	Start      string              `json:"start"`
	End        string              `json:"end"`
	Members    []hostbackup.Member `json:"members"`
	Chunks     map[string][]string `json:"chunks"`
}

func BaseBackupDestination(destination offsite.Destination) offsite.Destination {
	destination.ChunkFolder = baseBackupChunkFolder
	return destination
}

func (backup BaseBackup) StartLSN() (pglogrepl.LSN, error) {
	return pglogrepl.ParseLSN(backup.Start)
}

func (backup BaseBackup) EndLSN() (pglogrepl.LSN, error) {
	return pglogrepl.ParseLSN(backup.End)
}

func (backup BaseBackup) ChunkManifest() offsite.Manifest {
	return offsite.Manifest{Name: backup.Name, SealingKey: backup.SealingKey, CreatedAt: backup.CreatedAt, Archive: hostbackup.Manifest{Members: backup.Members}, Chunks: backup.Chunks}
}

func baseBackupPath(destination offsite.Destination, name string) string {
	return destination.CompanyID + "/" + baseBackupFolder + "/" + name
}

func TakeBaseBackup(ctx context.Context, replicationConnection string, destination offsite.Destination, keyring box.Keyring, now time.Time) (BaseBackup, error) {
	connection, errorValue := pgconn.Connect(ctx, replicationConnection)
	if errorValue != nil {
		return BaseBackup{}, fmt.Errorf("connecting to the company database for a base backup: %w", errorValue)
	}
	defer connection.Close(context.Background())
	if errorValue := requireStreamingBaseBackup(connection); errorValue != nil {
		return BaseBackup{}, errorValue
	}
	system, errorValue := pglogrepl.IdentifySystem(ctx, connection)
	if errorValue != nil {
		return BaseBackup{}, fmt.Errorf("identifying the company database for a base backup: %w", errorValue)
	}
	chunked := BaseBackupDestination(destination)
	snapshot, errorValue := offsite.StartSnapshot(ctx, chunked, keyring)
	if errorValue != nil {
		return BaseBackup{}, errorValue
	}
	started, errorValue := pglogrepl.StartBaseBackup(ctx, connection, pglogrepl.BaseBackupOptions{Label: "internkim standby", Fast: true, NoWait: true, Manifest: true})
	if errorValue != nil {
		return BaseBackup{}, fmt.Errorf("starting a base backup of the company database: %w", errorValue)
	}
	if len(started.Tablespaces) > 1 {
		return BaseBackup{}, fmt.Errorf("the company database keeps %d tablespaces; a base backup keeps the data directory alone, so a tablespace outside it would be lost", len(started.Tablespaces)-1)
	}
	if errorValue := pglogrepl.NextTableSpace(ctx, connection); errorValue != nil {
		return BaseBackup{}, fmt.Errorf("reading the base backup of the company database: %w", errorValue)
	}
	stream := &baseBackupStream{ctx: ctx, connection: connection}
	for {
		name, isMore, errorValue := stream.nextArchive()
		if errorValue != nil {
			return BaseBackup{}, errorValue
		}
		if !isMore {
			break
		}
		if errorValue := snapshot.AddMember(name, stream.copyArchive); errorValue != nil {
			return BaseBackup{}, errorValue
		}
	}
	finished, errorValue := pglogrepl.FinishBaseBackup(ctx, connection)
	if errorValue != nil {
		return BaseBackup{}, fmt.Errorf("finishing the base backup of the company database: %w", errorValue)
	}
	backup := BaseBackup{
		Version:    baseBackupVersion,
		Name:       baseBackupNamePrefix + now.UTC().Format(baseBackupTimeLayout) + baseBackupNameSuffix,
		SealingKey: destination.SealingKey,
		CreatedAt:  now.UTC(),
		SystemID:   system.SystemID,
		Timeline:   started.TimelineID,
		Start:      started.LSN.String(),
		End:        finished.LSN.String(),
		Members:    snapshot.Members(),
		Chunks:     snapshot.Chunks(),
	}
	if !slices.ContainsFunc(backup.Members, func(member hostbackup.Member) bool { return member.Name == DataDirectoryArchive }) {
		return BaseBackup{}, fmt.Errorf("the base backup of the company database sent no %s", DataDirectoryArchive)
	}
	document, errorValue := json.Marshal(backup)
	if errorValue != nil {
		return BaseBackup{}, errorValue
	}
	return backup, destination.Store.Put(ctx, baseBackupPath(destination, backup.Name), document)
}

func requireStreamingBaseBackup(connection *pgconn.PgConn) error {
	version := connection.ParameterStatus("server_version")
	major, _, _ := strings.Cut(version, ".")
	var number int
	if _, errorValue := fmt.Sscanf(major, "%d", &number); errorValue != nil || number < oldestStreamingVersion {
		return fmt.Errorf("the company database runs PostgreSQL %s; keeping a base backup needs PostgreSQL %d or newer", version, oldestStreamingVersion)
	}
	return nil
}

type baseBackupStream struct {
	ctx        context.Context
	connection *pgconn.PgConn
	nextName   string
	isDone     bool
}

func (stream *baseBackupStream) nextArchive() (string, bool, error) {
	if stream.nextName != "" {
		name := stream.nextName
		stream.nextName = ""
		return name, true, nil
	}
	if stream.isDone {
		return "", false, nil
	}
	if errorValue := stream.copyArchive(io.Discard); errorValue != nil {
		return "", false, errorValue
	}
	return stream.nextArchive()
}

func (stream *baseBackupStream) copyArchive(output io.Writer) error {
	for {
		message, errorValue := stream.connection.ReceiveMessage(stream.ctx)
		if errorValue != nil {
			return fmt.Errorf("reading the base backup of the company database: %w", errorValue)
		}
		switch message := message.(type) {
		case *pgproto3.CopyDone:
			stream.isDone = true
			return nil
		case *pgproto3.ErrorResponse:
			return fmt.Errorf("the company database stopped its base backup: %w", pgconn.ErrorResponseToPgError(message))
		case *pgproto3.CopyData:
			isEnd, errorValue := stream.takeCopyData(message.Data, output)
			if errorValue != nil || isEnd {
				return errorValue
			}
		}
	}
}

func (stream *baseBackupStream) takeCopyData(data []byte, output io.Writer) (bool, error) {
	if len(data) == 0 {
		return false, nil
	}
	switch data[0] {
	case newArchiveMessage:
		name, _, _ := bytes.Cut(data[1:], []byte{0})
		stream.nextName = string(name)
		return true, nil
	case manifestMessage:
		stream.nextName = BackupManifestArchive
		return true, nil
	case archiveDataMessage:
		_, errorValue := output.Write(data[1:])
		return false, errorValue
	case progressMessage:
		return false, nil
	}
	return false, fmt.Errorf("the base backup of the company database sent a message of kind %q this box does not read", data[0])
}

func NewestBaseBackup(ctx context.Context, destination offsite.Destination) (BaseBackup, error) {
	names, errorValue := baseBackupNames(ctx, destination)
	if errorValue != nil {
		return BaseBackup{}, errorValue
	}
	for index := len(names) - 1; index >= 0; index-- {
		backup, isReadable := readBaseBackup(ctx, destination, names[index])
		if isReadable && backup.SealingKey == destination.SealingKey {
			return backup, nil
		}
	}
	return BaseBackup{}, ErrNoBaseBackup
}

func baseBackupNames(ctx context.Context, destination offsite.Destination) ([]string, error) {
	listed, errorValue := destination.Store.List(ctx, destination.CompanyID+"/"+baseBackupFolder+"/")
	if errorValue != nil {
		return nil, errorValue
	}
	names := []string{}
	for _, object := range listed {
		if strings.HasPrefix(object.Name, baseBackupNamePrefix) && strings.HasSuffix(object.Name, baseBackupNameSuffix) {
			names = append(names, object.Name)
		}
	}
	slices.Sort(names)
	return names, nil
}

func readBaseBackup(ctx context.Context, destination offsite.Destination, name string) (BaseBackup, bool) {
	document, errorValue := destination.Store.Get(ctx, baseBackupPath(destination, name))
	if errorValue != nil {
		return BaseBackup{}, false
	}
	var backup BaseBackup
	if json.Unmarshal(document, &backup) != nil || backup.Version != baseBackupVersion || backup.Name != name || backup.SystemID == "" {
		return BaseBackup{}, false
	}
	return backup, true
}
