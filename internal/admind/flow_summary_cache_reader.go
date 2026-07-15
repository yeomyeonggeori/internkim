package admind

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

type flowSummaryReadModelBuilder func(context.Context) (flowSummaryReadModel, error)

func (service *Service) readCachedFlowSummaryReadModel(ctx context.Context, weekCode string, keys flowSummaryDependencyKeys, memberFingerprint string, build flowSummaryReadModelBuilder) (flowSummaryReadModel, error) {
	entry, snapshot, found, cacheError := service.readFlowSummaryCacheSnapshot(ctx, weekCode, keys, memberFingerprint)
	if cacheError == nil && found {
		readModel, decodeError := decodeFlowSummaryCachePayload(entry.Payload)
		if decodeError == nil {
			return readModel, nil
		}
		if deleteError := service.deleteFlowSummaryCacheEntryIfUnchanged(ctx, entry); deleteError != nil {
			logFlowSummaryCacheFailure("delete corrupt payload", weekCode, deleteError)
		}
	} else if cacheError != nil {
		logFlowSummaryCacheFailure("read", weekCode, cacheError)
	}
	readModel, errorValue := build(ctx)
	if errorValue != nil {
		return flowSummaryReadModel{}, errorValue
	}
	if cacheError == nil {
		payload, encodeError := encodeFlowSummaryCachePayload(readModel)
		if encodeError != nil {
			logFlowSummaryCacheFailure("encode", weekCode, encodeError)
		} else {
			now := time.Now()
			stored, writeError := service.writeFlowSummaryCacheEntryIfCurrent(ctx, weekCode, keys, snapshot, payload, now)
			if writeError != nil {
				logFlowSummaryCacheFailure("write", weekCode, writeError)
			} else if stored {
				if cleanupError := service.cleanupFlowSummaryCacheEntries(ctx, now); cleanupError != nil {
					logFlowSummaryCacheFailure("cleanup", weekCode, cleanupError)
				}
			}
		}
	}
	return readModel, nil
}

func encodeFlowSummaryCachePayload(readModel flowSummaryReadModel) ([]byte, error) {
	payload := flowSummaryCachePayload{
		Version:     flowSummaryCacheSchemaVersion,
		WeeklyTasks: readModel.WeeklyTasks,
		Metrics:     readModel.Metrics,
		Report:      readModel.Report,
	}
	document, errorValue := json.Marshal(payload)
	if errorValue != nil {
		return nil, fmt.Errorf("encode flow summary cache payload: %w", errorValue)
	}
	return document, nil
}

func decodeFlowSummaryCachePayload(document string) (flowSummaryReadModel, error) {
	var payload flowSummaryCachePayload
	if errorValue := json.Unmarshal([]byte(document), &payload); errorValue != nil {
		return flowSummaryReadModel{}, fmt.Errorf("decode flow summary cache payload: %w", errorValue)
	}
	if payload.Version != flowSummaryCacheSchemaVersion {
		return flowSummaryReadModel{}, fmt.Errorf("decode flow summary cache payload: schema version %d", payload.Version)
	}
	return flowSummaryReadModel{WeeklyTasks: payload.WeeklyTasks, Metrics: payload.Metrics, Report: payload.Report}, nil
}

func logFlowSummaryCacheFailure(operation string, weekCode string, errorValue error) {
	slog.Warn(
		"flow summary cache operation failed",
		"operation", operation,
		"week_code", weekCode,
		"error", errorValue.Error(),
	)
}
