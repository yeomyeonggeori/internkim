package offsite

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/hostbackup"
)

const manifestContractPath = "testdata/manifest.json"

func TestTheSnapshotManifestIsTheOneTheWebAppReads(t *testing.T) {
	createdAt := time.Date(2026, 10, 19, 3, 0, 0, 0, time.UTC)
	sample := Manifest{
		Version:    manifestVersion,
		Name:       snapshotNameAt(createdAt),
		SealingKey: "c2FtcGxlLXNlYWxpbmcta2V5LXNhbXBsZS1zZWFsaW4",
		CreatedAt:  createdAt,
		Archive: hostbackup.Manifest{
			Format:          hostbackup.FormatName,
			FormatVersion:   hostbackup.FormatVersion,
			CreatedAt:       createdAt,
			PackageVersion:  "1:0.0.1",
			CompanyID:       companyID,
			PostgreSQLMajor: 17,
			FileRoots:       []string{"state"},
			Members:         []hostbackup.Member{{Name: hostbackup.RosterMemberName, Size: 2, SHA256: "00"}},
		},
		Chunks: map[string][]string{hostbackup.RosterMemberName: {"00"}},
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
		t.Fatalf("%s is not the manifest a snapshot writes; run with INTERNKIM_WRITE_CONTRACT=1 and update keptBackupManifestSchema in web/src/lib/company/box.ts to match:\n%s", manifestContractPath, written)
	}
}
