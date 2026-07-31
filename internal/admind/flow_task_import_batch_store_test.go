package admind

import "testing"

func runningFlowTaskImportBatch() flowTaskImportBatch {
	return flowTaskImportBatch{
		BatchID:        "legacy-notion-20260730-v1-part-001",
		SourceSystem:   "notion",
		RequesterEmail: "admin@example.com",
		Status:         flowTaskImportBatchStatusRunning,
		IsDryRun:       true,
		ConflictPolicy: "skip",
		ReceivedCount:  250,
		ValidCount:     247,
		StartedAt:      "2026-07-31T00:00:00Z",
	}
}

func TestFlowTaskImportBatchRoundTrips(t *testing.T) {
	ctx, database := newFlowProvenanceTestDatabase(t)
	batch := runningFlowTaskImportBatch()
	if errorValue := writeFlowTaskImportBatch(ctx, database, batch); errorValue != nil {
		t.Fatal(errorValue)
	}

	stored, found, errorValue := readFlowTaskImportBatch(ctx, database, batch.BatchID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if !found {
		t.Fatal("batch was not found")
	}
	if !stored.IsDryRun || stored.SourceSystem != "notion" || stored.ReceivedCount != 250 || stored.ValidCount != 247 {
		t.Fatalf("stored = %+v", stored)
	}
	if stored.Status != flowTaskImportBatchStatusRunning || stored.StartedAt != "2026-07-31T00:00:00Z" || stored.FinishedAt != "" {
		t.Fatalf("stored = %+v", stored)
	}
}

func TestFlowTaskImportBatchRecordsFinalTally(t *testing.T) {
	ctx, database := newFlowProvenanceTestDatabase(t)
	batch := runningFlowTaskImportBatch()
	if errorValue := writeFlowTaskImportBatch(ctx, database, batch); errorValue != nil {
		t.Fatal(errorValue)
	}

	batch.Status = flowTaskImportBatchStatusCompleted
	batch.IsDryRun = false
	batch.CreatedCount = 240
	batch.SkippedCount = 5
	batch.ConflictCount = 1
	batch.FailedCount = 4
	batch.PotentialDuplicateCount = 3
	batch.FinishedAt = "2026-07-31T00:05:00Z"
	if errorValue := writeFlowTaskImportBatch(ctx, database, batch); errorValue != nil {
		t.Fatal(errorValue)
	}

	stored, _, errorValue := readFlowTaskImportBatch(ctx, database, batch.BatchID)
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if stored.Status != flowTaskImportBatchStatusCompleted || stored.FinishedAt != "2026-07-31T00:05:00Z" {
		t.Fatalf("stored = %+v", stored)
	}
	tally := stored.CreatedCount + stored.UpdatedCount + stored.SkippedCount + stored.FailedCount
	if tally != stored.ReceivedCount-1 {
		t.Fatalf("tally = %d received = %d stored = %+v", tally, stored.ReceivedCount, stored)
	}
	if stored.StartedAt != "2026-07-31T00:00:00Z" {
		t.Fatalf("rewriting the batch must not move startedAt: %+v", stored)
	}
}

func TestFlowTaskImportBatchRequiresIdentity(t *testing.T) {
	ctx, database := newFlowProvenanceTestDatabase(t)
	for _, testCase := range []struct {
		name  string
		batch flowTaskImportBatch
	}{
		{name: "no batch", batch: flowTaskImportBatch{SourceSystem: "notion", Status: flowTaskImportBatchStatusRunning}},
		{name: "no source system", batch: flowTaskImportBatch{BatchID: "batch-1", Status: flowTaskImportBatchStatusRunning}},
		{name: "no status", batch: flowTaskImportBatch{BatchID: "batch-1", SourceSystem: "notion"}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			if errorValue := writeFlowTaskImportBatch(ctx, database, testCase.batch); errorValue != errFlowTaskImportBatchIncomplete {
				t.Fatalf("error = %v", errorValue)
			}
		})
	}
}

func TestFlowTaskImportBatchMissingIsNotAnError(t *testing.T) {
	ctx, database := newFlowProvenanceTestDatabase(t)
	stored, found, errorValue := readFlowTaskImportBatch(ctx, database, "batch-404")
	if errorValue != nil {
		t.Fatal(errorValue)
	}
	if found {
		t.Fatalf("stored = %+v", stored)
	}
}
