package walstream

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"maps"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgproto3"
)

var (
	sampleDataDirectoryTar = []byte("sample data directory tar")
	sampleBackupManifest   = []byte("{\"PostgreSQL-Backup-Manifest-Version\": 1}")
)

type fakePrimary struct {
	version     string
	tablespaces [][]byte
	archives    func(backend *pgproto3.Backend)
}

func TestPostgreSQL14KeepsABaseBackupOfItsDataDirectoryAndManifest(t *testing.T) {
	primary := fakePrimary{version: "14.22 (Ubuntu 14.22-0ubuntu0.22.04.1)", archives: sendACopyPerArchive}
	archives, query, streamed := streamFrom(t, primary)
	if query != "BASE_BACKUP LABEL 'internkim standby' FAST NOWAIT MANIFEST 'yes'" {
		t.Errorf("PostgreSQL 14 was asked %q, which is not the BASE_BACKUP grammar it reads", query)
	}
	assertTheDataDirectoryAndManifestWereKept(t, archives, streamed)
}

func TestPostgreSQL15AndNewerKeepABaseBackupFromOneTaggedCopyStream(t *testing.T) {
	primary := fakePrimary{version: "17.6", archives: sendOneTaggedCopy}
	archives, query, streamed := streamFrom(t, primary)
	if !strings.HasPrefix(query, "BASE_BACKUP(") {
		t.Errorf("PostgreSQL 17 was asked %q, which is not the BASE_BACKUP grammar it reads", query)
	}
	assertTheDataDirectoryAndManifestWereKept(t, archives, streamed)
}

func TestABaseBackupOfADatabaseWithATablespaceOutsideItsDataDirectoryIsRefused(t *testing.T) {
	primary := fakePrimary{version: "17.6", tablespaces: [][]byte{[]byte("16384"), []byte("/srv/elsewhere"), nil}, archives: sendOneTaggedCopy}
	connection, _ := connectTo(t, primary)
	_, errorValue := streamBaseBackup(context.Background(), connection, 17, keepNothing)
	if errorValue == nil || !strings.Contains(errorValue.Error(), "1 tablespaces outside its data directory") {
		t.Fatalf("a base backup that would lose a tablespace was not refused: %v", errorValue)
	}
}

func TestAPostgreSQLWithoutABackupManifestIsRefusedByName(t *testing.T) {
	connection, _ := connectTo(t, fakePrimary{version: "12.20"})
	_, errorValue := streamingMajor(connection)
	if errorValue == nil || !strings.Contains(errorValue.Error(), "PostgreSQL 12.20") {
		t.Fatalf("PostgreSQL 12 was not refused by name: %v", errorValue)
	}
}

func streamFrom(t *testing.T, primary fakePrimary) (map[string][]byte, string, streamedBaseBackup) {
	t.Helper()
	connection, queries := connectTo(t, primary)
	major, errorValue := streamingMajor(connection)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	archives := map[string][]byte{}
	keep := func(name string, produce func(io.Writer) error) error {
		var content bytes.Buffer
		errorValue := produce(&content)
		archives[name] = content.Bytes()
		return errorValue
	}
	streamed, errorValue := streamBaseBackup(context.Background(), connection, major, keep)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	return archives, <-queries, streamed
}

func assertTheDataDirectoryAndManifestWereKept(t *testing.T, archives map[string][]byte, streamed streamedBaseBackup) {
	t.Helper()
	wanted := map[string][]byte{DataDirectoryArchive: sampleDataDirectoryTar, BackupManifestArchive: sampleBackupManifest}
	if !maps.EqualFunc(archives, wanted, bytes.Equal) {
		t.Errorf("the base backup kept %q, wanted %q", archives, wanted)
	}
	if streamed.started.LSN.String() != "0/2000028" || streamed.finished.LSN.String() != "0/2000100" || streamed.started.TimelineID != 1 {
		t.Errorf("the base backup ran from %s to %s on timeline %d", streamed.started.LSN, streamed.finished.LSN, streamed.started.TimelineID)
	}
}

func keepNothing(string, func(io.Writer) error) error {
	return nil
}

func connectTo(t *testing.T, primary fakePrimary) (*pgconn.PgConn, <-chan string) {
	t.Helper()
	listener, errorValue := net.Listen("tcp", "127.0.0.1:0")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	t.Cleanup(func() { listener.Close() })
	queries := make(chan string, 1)
	go primary.serve(listener, queries)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	address := fmt.Sprintf("host=127.0.0.1 port=%d user=sample dbname=postgres sslmode=disable", listener.Addr().(*net.TCPAddr).Port)
	connection, errorValue := pgconn.Connect(ctx, address)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	t.Cleanup(func() { connection.Close(context.Background()) })
	return connection, queries
}

func (primary fakePrimary) serve(listener net.Listener, queries chan<- string) {
	connection, errorValue := listener.Accept()
	if errorValue != nil {
		return
	}
	defer connection.Close()
	backend := pgproto3.NewBackend(connection, connection)
	if _, errorValue := backend.ReceiveStartupMessage(); errorValue != nil {
		return
	}
	backend.Send(&pgproto3.AuthenticationOk{})
	backend.Send(&pgproto3.ParameterStatus{Name: "server_version", Value: primary.version})
	backend.Send(&pgproto3.BackendKeyData{ProcessID: 1, SecretKey: []byte{0, 0, 0, 1}})
	backend.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
	if backend.Flush() != nil {
		return
	}
	message, errorValue := backend.Receive()
	query, isQuery := message.(*pgproto3.Query)
	if errorValue != nil || !isQuery {
		return
	}
	queries <- query.String
	sendPosition(backend, "0/2000028")
	sendTablespaces(backend, primary.tablespaces)
	primary.archives(backend)
	sendPosition(backend, "0/2000100")
	backend.Send(&pgproto3.CommandComplete{CommandTag: []byte("BASE_BACKUP")})
	backend.Send(&pgproto3.ReadyForQuery{TxStatus: 'I'})
	backend.Flush()
}

func sendPosition(backend *pgproto3.Backend, position string) {
	backend.Send(&pgproto3.RowDescription{Fields: []pgproto3.FieldDescription{{Name: []byte("recptr")}, {Name: []byte("tli")}}})
	backend.Send(&pgproto3.DataRow{Values: [][]byte{[]byte(position), []byte("1")}})
	backend.Send(&pgproto3.CommandComplete{CommandTag: []byte("SELECT 1")})
}

func sendTablespaces(backend *pgproto3.Backend, extra [][]byte) {
	backend.Send(&pgproto3.RowDescription{Fields: []pgproto3.FieldDescription{{Name: []byte("spcoid")}, {Name: []byte("spclocation")}, {Name: []byte("size")}}})
	if extra != nil {
		backend.Send(&pgproto3.DataRow{Values: extra})
	}
	backend.Send(&pgproto3.DataRow{Values: [][]byte{nil, nil, nil}})
	backend.Send(&pgproto3.CommandComplete{CommandTag: []byte("SELECT")})
}

func sendACopyPerArchive(backend *pgproto3.Backend) {
	for _, content := range [][]byte{sampleDataDirectoryTar, sampleBackupManifest} {
		backend.Send(&pgproto3.CopyOutResponse{})
		backend.Send(&pgproto3.CopyData{Data: content[:4]})
		backend.Send(&pgproto3.CopyData{Data: content[4:]})
		backend.Send(&pgproto3.CopyDone{})
	}
}

func sendOneTaggedCopy(backend *pgproto3.Backend) {
	backend.Send(&pgproto3.CopyOutResponse{})
	backend.Send(&pgproto3.CopyData{Data: append([]byte{newArchiveMessage}, DataDirectoryArchive+"\x00\x00"...)})
	backend.Send(&pgproto3.CopyData{Data: append([]byte{archiveDataMessage}, sampleDataDirectoryTar...)})
	backend.Send(&pgproto3.CopyData{Data: append([]byte{progressMessage}, make([]byte, 8)...)})
	backend.Send(&pgproto3.CopyData{Data: []byte{manifestMessage}})
	backend.Send(&pgproto3.CopyData{Data: append([]byte{archiveDataMessage}, sampleBackupManifest...)})
	backend.Send(&pgproto3.CopyDone{})
}
