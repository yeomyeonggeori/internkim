package offsite

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

const manifestContractPath = "testdata/manifest.json"

func TestTheKeptManifestIsTheOneTheWebAppReads(t *testing.T) {
	sample := Manifest{
		Version:     manifestVersion,
		ArchiveName: "internkim-backup-20261019T030000Z.tar",
		SealingKey:  "c2FtcGxlLXNlYWxpbmcta2V5LXNhbXBsZS1zZWFsaW4",
		CreatedAt:   time.Date(2026, 10, 19, 3, 0, 0, 0, time.UTC),
		Parts:       []Part{{Name: "part-000001", Size: 1024, SHA256: "00"}},
	}
	written, errorValue := json.MarshalIndent(sample, "", "  ")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	written = append(written, '\n')
	if os.Getenv("INTERNKIM_WRITE_CONTRACT") == "1" {
		if errorValue := os.WriteFile(manifestContractPath, written, 0o644); errorValue != nil {
			t.Fatal(errorValue)
		}
	}
	kept, errorValue := os.ReadFile(manifestContractPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if string(kept) != string(written) {
		t.Fatalf("%s is not the manifest Ship writes; run with INTERNKIM_WRITE_CONTRACT=1 and update keptBackupManifestSchema in web/src/lib/company/box.ts to match:\n%s", manifestContractPath, written)
	}
}
