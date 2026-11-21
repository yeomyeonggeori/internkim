package walstream

import (
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/yeomyeonggeori/internkim/internal/hostbackup"
)

const baseBackupContractPath = "testdata/base-backup.json"

func TestTheBaseBackupDocumentIsTheOneTheWebAppReads(t *testing.T) {
	createdAt := time.Date(2026, 11, 22, 3, 0, 0, 0, time.UTC)
	sample := BaseBackup{
		Version:    baseBackupVersion,
		Name:       baseBackupNamePrefix + createdAt.Format(baseBackupTimeLayout) + baseBackupNameSuffix,
		SealingKey: "c2FtcGxlLXNlYWxpbmcta2V5LXNhbXBsZS1zZWFsaW4",
		CreatedAt:  createdAt,
		SystemID:   "7300000000000000001",
		Timeline:   1,
		Start:      "0/2000028",
		End:        "0/2000100",
		Members:    []hostbackup.Member{{Name: DataDirectoryArchive, Size: 2, SHA256: "00"}},
		Chunks:     map[string][]string{DataDirectoryArchive: {"00"}},
	}
	written, errorValue := json.MarshalIndent(sample, "", "  ")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	written = append(written, '\n')
	kept, errorValue := os.ReadFile(baseBackupContractPath)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if string(kept) != string(written) {
		t.Fatalf("%s is not the document a base backup writes; write it there and update keptBaseBackupSchema in web/src/lib/company/box.ts to match:\n%s", baseBackupContractPath, written)
	}
}
