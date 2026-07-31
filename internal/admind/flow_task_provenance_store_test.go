package admind

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
)

func newFlowProvenanceTestDatabase(t *testing.T) (context.Context, *sql.DB) {
	t.Helper()
	ctx := context.Background()
	service := NewService(Configuration{DatabasePath: filepath.Join(t.TempDir(), "internkim.sqlite")})
	database, errorValue := service.openFlowDatabase(ctx)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	t.Cleanup(func() { database.Close() })
	return ctx, database
}

func sampleFlowTaskExternalReference() flowTaskExternalReference {
	return flowTaskExternalReference{
		TaskID:                "task-1",
		SourceSystem:          "notion",
		SourceRecordID:        "TSK-7",
		SourceLocator:         "notion/control_tower/tasks.csv#row=2",
		SourcePayloadHash:     "sha256:aaa",
		DedupeKey:             "sha256:bbb",
		ImportBatchID:         "legacy-notion-part-001",
		OriginalCreatedAt:     "2024-04-22T14:09+09:00",
		OriginalUpdatedAt:     "2024-12-03T17:52+09:00",
		OriginalOwnerName:     "민성 김",
		OriginalStatus:        "시작 전",
		OriginalBusiness:      "기본소득",
		OriginalCategory:      "모바일",
		OriginalType:          "작업",
		SourceMetadata:        `{"priority":"중간"}`,
		NormalizationWarnings: []string{"MISSING_STATUS", "END_BEFORE_START"},
		CreatedAt:             "2026-07-31T00:00:00Z",
	}
}

func TestFlowTaskProvenanceSchemaAddsHistoricalColumns(t *testing.T) {
	ctx, database := newFlowProvenanceTestDatabase(t)
	for _, columnName := range []string{"is_historical", "archived_at"} {
		var count int
		if errorValue := database.QueryRowContext(ctx, "SELECT COUNT(*) FROM pragma_table_info('flow_tasks') WHERE name = ?", columnName).Scan(&count); errorValue != nil {
			t.Fatal(errorValue)
		}
		if count != 1 {
			t.Fatalf("flow_tasks column %q count = %d", columnName, count)
		}
	}
}

func TestFlowTaskExternalReferenceRoundTrips(t *testing.T) {
	ctx, database := newFlowProvenanceTestDatabase(t)
	reference := sampleFlowTaskExternalReference()
	if errorValue := writeFlowTaskExternalReference(ctx, database, reference); errorValue != nil {
		t.Fatal(errorValue)
	}

	stored, found, errorValue := readFlowTaskExternalReferenceBySource(ctx, database, "notion", "TSK-7")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !found {
		t.Fatal("external reference was not found")
	}
	if stored.TaskID != reference.TaskID || stored.OriginalOwnerName != "민성 김" || stored.OriginalStatus != "시작 전" {
		t.Fatalf("stored = %+v", stored)
	}
	if stored.OriginalBusiness != "기본소득" || stored.OriginalCategory != "모바일" || stored.OriginalType != "작업" {
		t.Fatalf("mapping provenance lost: %+v", stored)
	}
	if strings.Join(stored.NormalizationWarnings, ",") != "MISSING_STATUS,END_BEFORE_START" {
		t.Fatalf("warnings = %+v", stored.NormalizationWarnings)
	}
}

func TestFlowTaskExternalReferenceRejectsDuplicateSourceRecord(t *testing.T) {
	ctx, database := newFlowProvenanceTestDatabase(t)
	reference := sampleFlowTaskExternalReference()
	if errorValue := writeFlowTaskExternalReference(ctx, database, reference); errorValue != nil {
		t.Fatal(errorValue)
	}
	reference.TaskID = "task-2"
	errorValue := writeFlowTaskExternalReference(ctx, database, reference)
	if errorValue == nil {
		t.Fatal("re-importing the same source record must not create a second task")
	}
	if !strings.Contains(strings.ToLower(errorValue.Error()), "unique") {
		t.Fatalf("error = %v", errorValue)
	}
}

func TestFlowTaskExternalReferenceRequiresSourceIdentity(t *testing.T) {
	ctx, database := newFlowProvenanceTestDatabase(t)
	for _, testCase := range []struct {
		name      string
		reference flowTaskExternalReference
	}{
		{name: "no task", reference: flowTaskExternalReference{SourceSystem: "notion", SourceRecordID: "TSK-7"}},
		{name: "no source system", reference: flowTaskExternalReference{TaskID: "task-1", SourceRecordID: "TSK-7"}},
		{name: "no source record", reference: flowTaskExternalReference{TaskID: "task-1", SourceSystem: "notion"}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if errorValue := writeFlowTaskExternalReference(ctx, database, testCase.reference); errorValue != errFlowTaskExternalReferenceIncomplete {
				t.Fatalf("error = %v", errorValue)
			}
		})
	}
}

func TestFlowTaskExternalReferencesReadByBatch(t *testing.T) {
	ctx, database := newFlowProvenanceTestDatabase(t)
	first := sampleFlowTaskExternalReference()
	second := sampleFlowTaskExternalReference()
	second.TaskID = "task-2"
	second.SourceRecordID = "TSK-8"
	other := sampleFlowTaskExternalReference()
	other.TaskID = "task-3"
	other.SourceRecordID = "GSH-1"
	other.SourceSystem = "google_sheets"
	other.ImportBatchID = "legacy-sheets-part-001"
	for _, reference := range []flowTaskExternalReference{first, second, other} {
		if errorValue := writeFlowTaskExternalReference(ctx, database, reference); errorValue != nil {
			t.Fatal(errorValue)
		}
	}

	references, errorValue := readFlowTaskExternalReferencesByBatch(ctx, database, "legacy-notion-part-001")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if len(references) != 2 {
		t.Fatalf("references = %+v", references)
	}
	for _, reference := range references {
		if reference.ImportBatchID != "legacy-notion-part-001" {
			t.Fatalf("batch leaked another batch's record: %+v", reference)
		}
	}
}

func TestFlowTaskExternalReferenceMissingSourceIsNotAnError(t *testing.T) {
	ctx, database := newFlowProvenanceTestDatabase(t)
	stored, found, errorValue := readFlowTaskExternalReferenceBySource(ctx, database, "notion", "TSK-404")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if found {
		t.Fatalf("stored = %+v", stored)
	}
}
