package deviceexport

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/klauspost/compress/zstd"
	"github.com/yeomyeonggeori/internkim/internal/hostbackup"
)

const (
	sampleCompanyID = "00000000-0000-4000-8000-0000000000aa"
	samplePersonID  = "00000000-0000-4000-8000-0000000000bb"
	sampleSeed      = "1111111111111111111111111111111111111111111111111111111111111111"
	sampleRelayKey  = "2222222222222222222222222222222222222222222222222222222222222222"
	sampleAgentKey  = "3333333333333333333333333333333333333333333333333333333333333333"
)

type fixtureEntry struct {
	name     string
	body     string
	typeflag byte
	uid      int
	gid      int
	linkname string
}

func TestAnExportBecomesAnArchiveTheRestoreReads(t *testing.T) {
	exported := writeFixtureExport(t)
	archivePath := filepath.Join(t.TempDir(), "jetson.tar")
	report, errorValue := Convert(Request{
		ExportDirectoryPath: exported,
		ConnectionPath:      filepath.Join(exported, "connection.json"),
		HostPasswdPath:      filepath.Join(exported, "host-passwd"),
		HostGroupPath:       filepath.Join(exported, "host-group"),
		ArchivePath:         archivePath,
		CreatedAt:           time.Date(2026, 10, 1, 14, 37, 57, 0, time.UTC),
	})
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	archive, errorValue := hostbackup.Open(archivePath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := archive.Verify(); errorValue != nil {
		t.Fatal(errorValue)
	}
	manifest := archive.Manifest
	if manifest.CompanyID != sampleCompanyID || manifest.PostgreSQLMajor != 15 {
		t.Fatalf("manifest = %+v", manifest)
	}
	for _, member := range []string{"databases/blueclaw.dump", "databases/buzz.dump", "roster.json", "files.tar"} {
		if _, isCarried := manifest.Member(member); !isCarried {
			t.Errorf("the archive carries no %s", member)
		}
	}
	if readMember(t, archive, "databases/buzz.dump") != "buzz dump" || readMember(t, archive, "roster.json") != `{"people":[]}` {
		t.Error("the buzz dump or the roster is not the device's")
	}

	entries := filesEntries(t, archive)
	expectEntry(t, entries, "state/companies/"+sampleCompanyID+"/secrets/buzz-key-seed", "root", "root", sampleSeed+"\n")
	expectEntry(t, entries, "state/companies/"+sampleCompanyID+"/secrets/buzz-relay-key", "root", "root", sampleRelayKey+"\n")
	expectEntry(t, entries, "state/companies/"+sampleCompanyID+"/secrets/openrouter-key", "root", "root", "sk-or-sample\n")
	expectEntry(t, entries, "state/chatd/state.json", "root", "root", "chatd")
	expectEntry(t, entries, "state/buzz-account-links.json", "root", "root", "links")
	expectEntry(t, entries, "administration/state/admin/buzz-account-links.json", "root", "root", "links")
	expectEntry(t, entries, "administration/sites/abc/site.db", "internkim-site", "internkim-site", "site")
	expectEntry(t, entries, "administration/secrets/sites/abc", "root", "root", "site secret")
	expectEntry(t, entries, "relay/inbound/one.json", "internkim", "internkim", "queued")
	expectEntry(t, entries, "state/media/buzz-media/aa/0123", "root", "root", "png")
	expectEntry(t, entries, "workspace/private/people/"+samplePersonID+"/note.md", "bc_person_sample", "bc_person_sample", "hello")
	expectEntry(t, entries, "workspace/circles/staff/plan.md", "blueclaw", "bc_circle_staff", "plan")
	expectEntry(t, entries, "workspace/.blueclaw/identity-map.json", "root", "root", "")
	if link := entries["workspace/circles/staff/plan-copy.md"]; link == nil || link.header.Linkname != "workspace/circles/staff/plan.md" {
		t.Errorf("a hard link must name its target under the same root, got %+v", link)
	}
	for _, leftOut := range []string{
		"administration/models/big.gguf", "administration/env/fleet-id", "administration/secrets/fleet-secret",
		"administration/backups/old.tar.gz", "workspace/shared/cache/pip/wheel", "workspace/.blueclaw/postgres/data/PG_VERSION",
	} {
		if entries[leftOut] != nil {
			t.Errorf("%s must not be carried", leftOut)
		}
	}
	if report.WorkspaceOwners["bc_person_sample"] == 0 {
		t.Errorf("the report counts no file of the sample person: %v", report.WorkspaceOwners)
	}
}

func TestAnExportWithoutTheMessengerSeedIsRefused(t *testing.T) {
	exported := writeFixtureExport(t)
	writeZstdTar(t, filepath.Join(exported, hostStateArchiveName), []fixtureEntry{
		{name: "var/lib/blueclaw/delivery/config/policy.json", body: `{"people":[]}`},
	})
	_, errorValue := Convert(Request{
		ExportDirectoryPath: exported,
		ConnectionPath:      filepath.Join(exported, "connection.json"),
		HostPasswdPath:      filepath.Join(exported, "host-passwd"),
		HostGroupPath:       filepath.Join(exported, "host-group"),
		ArchivePath:         filepath.Join(t.TempDir(), "jetson.tar"),
	})
	if errorValue == nil || !strings.Contains(errorValue.Error(), "seed") {
		t.Fatalf("an export that would change every person's messenger key must be refused, got %v", errorValue)
	}
}

type archivedEntry struct {
	header *tar.Header
	body   string
}

func expectEntry(t *testing.T, entries map[string]*archivedEntry, name string, userName string, groupName string, body string) {
	t.Helper()
	entry := entries[name]
	if entry == nil {
		t.Errorf("%s is missing", name)
		return
	}
	if entry.header.Uname != userName || entry.header.Gname != groupName {
		t.Errorf("%s is owned by %s:%s, want %s:%s", name, entry.header.Uname, entry.header.Gname, userName, groupName)
	}
	if body != "" && entry.body != body {
		t.Errorf("%s holds %q, want %q", name, entry.body, body)
	}
}

func readMember(t *testing.T, archive hostbackup.Archive, name string) string {
	t.Helper()
	var contents bytes.Buffer
	if errorValue := archive.ReadMember(name, func(input io.Reader) error {
		_, errorValue := io.Copy(&contents, input)
		return errorValue
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	return contents.String()
}

func filesEntries(t *testing.T, archive hostbackup.Archive) map[string]*archivedEntry {
	t.Helper()
	entries := map[string]*archivedEntry{}
	if errorValue := archive.ReadMember(hostbackup.FilesMemberName, func(input io.Reader) error {
		reader := tar.NewReader(input)
		for {
			header, errorValue := reader.Next()
			if errors.Is(errorValue, io.EOF) {
				return nil
			}
			if errorValue != nil {
				return errorValue
			}
			body, errorValue := io.ReadAll(reader)
			if errorValue != nil {
				return errorValue
			}
			entries[strings.TrimSuffix(header.Name, "/")] = &archivedEntry{header: header, body: string(body)}
		}
	}); errorValue != nil {
		t.Fatal(errorValue)
	}
	return entries
}

func writeFixtureExport(t *testing.T) string {
	t.Helper()
	exported := t.TempDir()
	files := map[string]string{
		"blueclaw.dump":            "blueclaw dump",
		"host-buzz.dump":           "buzz dump",
		"host-mattermost.dump":     "retired",
		"guest-postgres-version":   "15\n",
		"guest-passwd":             "root:x:0:0:root:/root:/bin/bash\nsystemd-network:x:998:998::/:/usr/sbin/nologin\nblueclaw:x:998:971::/home/blueclaw:/bin/sh\n",
		"guest-group":              "root:x:0:\nblueclaw:x:971:\n",
		"host-passwd":              "root:0\ninternkim:65543\ninternkim-site:995\nblueclaw:998\n",
		"host-group":               "root:0\ninternkim:65543\ninternkim-site:968\nblueclaw:971\n",
		"media/buzz-media/aa/0123": "png",
	}
	for name, body := range files {
		path := filepath.Join(exported, name)
		if errorValue := os.MkdirAll(filepath.Dir(path), 0o755); errorValue != nil {
			t.Fatal(errorValue)
		}
		if errorValue := os.WriteFile(path, []byte(body), 0o644); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	connection, _ := json.Marshal(map[string]any{
		"schemaVersion": 1, "appURL": "http://192.0.2.1",
		"company":      map[string]string{"id": sampleCompanyID, "name": "Sample Company", "slug": "sample"},
		"centralPlane": map[string]string{"projectURL": "http://192.0.2.1:54321", "publishableKey": "sb_publishable_sample"},
		"gatewayURL":   "ws://192.0.2.1:8787", "agentKey": sampleAgentKey,
	})
	if errorValue := os.WriteFile(filepath.Join(exported, "connection.json"), connection, 0o600); errorValue != nil {
		t.Fatal(errorValue)
	}
	writeZstdTar(t, filepath.Join(exported, hostStateArchiveName), []fixtureEntry{
		{name: "root/.internkim/", typeflag: tar.TypeDir},
		{name: "root/.internkim/secrets/buzz-key-seed", body: sampleSeed + "\n"},
		{name: "root/.internkim/secrets/buzz-relay-env", body: "BUZZ_RELAY_PRIVATE_KEY=" + sampleRelayKey + "\nRELAY_OWNER_PUBKEY=abc\n"},
		{name: "root/.internkim/secrets/openrouter-api-key", body: "sk-or-sample\n"},
		{name: "root/.internkim/secrets/fleet-secret", body: "never carried"},
		{name: "root/.internkim/secrets/sites/abc", body: "site secret"},
		{name: "root/.internkim/env/fleet-id", body: "fleet"},
		{name: "root/.internkim/models/big.gguf", body: "weights"},
		{name: "root/.internkim/backups/old.tar.gz", body: "old"},
		{name: "root/.internkim/sites/abc/site.db", body: "site", uid: 995, gid: 968},
		{name: "root/.internkim/state/chatd/state.json", body: "chatd"},
		{name: "root/.internkim/state/admin/buzz-account-links.json", body: "links"},
		{name: "var/lib/internkim/relay/inbound/one.json", body: "queued", uid: 65543, gid: 65543},
		{name: "var/lib/blueclaw/delivery/config/policy.json", body: `{"people":[]}`},
	})
	identityMap := `{"bc_person_sample":100003,"bc_circle_staff":100001,"bc_shared":100000}`
	writeZstdTar(t, filepath.Join(exported, workspaceArchiveName), []fixtureEntry{
		{name: "./", typeflag: tar.TypeDir, uid: 998, gid: 971},
		{name: "./.blueclaw/identity-map.json", body: identityMap},
		{name: "./.blueclaw/postgres/data/PG_VERSION", body: "15", uid: 100, gid: 102},
		{name: "./private/people/" + samplePersonID + "/note.md", body: "hello", uid: 100003, gid: 100003},
		{name: "./circles/staff/plan.md", body: "plan", uid: 998, gid: 100001},
		{name: "./circles/staff/plan-copy.md", typeflag: tar.TypeLink, linkname: "./circles/staff/plan.md", uid: 998, gid: 100001},
		{name: "./shared/cache/pip/wheel", body: "cache", uid: 998, gid: 100000},
	})
	return exported
}

func writeZstdTar(t *testing.T, path string, entries []fixtureEntry) {
	t.Helper()
	file, errorValue := os.Create(path)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	defer file.Close()
	encoder, errorValue := zstd.NewWriter(file)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	writer := tar.NewWriter(encoder)
	for _, entry := range entries {
		typeflag := entry.typeflag
		if typeflag == 0 {
			typeflag = tar.TypeReg
		}
		header := &tar.Header{Typeflag: typeflag, Name: entry.name, Mode: 0o640, Uid: entry.uid, Gid: entry.gid, Linkname: entry.linkname, ModTime: time.Unix(1_790_000_000, 0)}
		if typeflag == tar.TypeReg {
			header.Size = int64(len(entry.body))
		}
		if errorValue := writer.WriteHeader(header); errorValue != nil {
			t.Fatal(errorValue)
		}
		if typeflag == tar.TypeReg {
			if _, errorValue := writer.Write([]byte(entry.body)); errorValue != nil {
				t.Fatal(errorValue)
			}
		}
	}
	if errorValue := writer.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
	if errorValue := encoder.Close(); errorValue != nil {
		t.Fatal(errorValue)
	}
}
