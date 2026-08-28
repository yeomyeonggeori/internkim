package admind

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"
)

type taskSummaryReadModelBuilder func(context.Context) (taskSummaryReadModel, error)

func (service *Service) readCachedTaskSummaryReadModel(ctx context.Context, weekCode string, keys taskSummaryDependencyKeys, memberFingerprint string, build taskSummaryReadModelBuilder) (taskSummaryReadModel, error) {
	entry, snapshot, found, cacheError := service.readTaskSummaryCacheSnapshot(ctx, weekCode, keys, memberFingerprint)
	if cacheError == nil && found {
		readModel, decodeError := decodeTaskSummaryCachePayload(entry.Payload)
		if decodeError == nil {
			return readModel, nil
		}
		if deleteError := service.deleteTaskSummaryCacheEntryIfUnchanged(ctx, entry); deleteError != nil {
			logTaskSummaryCacheFailure("delete corrupt payload", weekCode, deleteError)
		}
	} else if cacheError != nil {
		logTaskSummaryCacheFailure("read", weekCode, cacheError)
	}
	readModel, errorValue := build(ctx)
	if errorValue != nil {
		return taskSummaryReadModel{}, errorValue
	}
	if cacheError == nil {
		payload, encodeError := encodeTaskSummaryCachePayload(readModel)
		if encodeError != nil {
			logTaskSummaryCacheFailure("encode", weekCode, encodeError)
		} else {
			now := time.Now()
			stored, writeError := service.writeTaskSummaryCacheEntryIfCurrent(ctx, weekCode, keys, snapshot, payload, now)
			if writeError != nil {
				logTaskSummaryCacheFailure("write", weekCode, writeError)
			} else if stored {
				if cleanupError := service.cleanupTaskSummaryCacheEntries(ctx, now); cleanupError != nil {
					logTaskSummaryCacheFailure("cleanup", weekCode, cleanupError)
				}
			}
		}
	}
	return readModel, nil
}

func encodeTaskSummaryCachePayload(readModel taskSummaryReadModel) ([]byte, error) {
	payload := taskSummaryCachePayload{
		Version:     taskSummaryCacheSchemaVersion,
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

func decodeTaskSummaryCachePayload(document string) (taskSummaryReadModel, error) {
	var payload taskSummaryCachePayload
	if errorValue := json.Unmarshal([]byte(document), &payload); errorValue != nil {
		return taskSummaryReadModel{}, fmt.Errorf("decode flow summary cache payload: %w", errorValue)
	}
	if payload.Version != taskSummaryCacheSchemaVersion {
		return taskSummaryReadModel{}, fmt.Errorf("decode flow summary cache payload: schema version %d", payload.Version)
	}
	return taskSummaryReadModel{WeeklyTasks: payload.WeeklyTasks, Metrics: payload.Metrics, Report: payload.Report}, nil
}

func logTaskSummaryCacheFailure(operation string, weekCode string, errorValue error) {
	slog.Warn(
		"flow summary cache operation failed",
		"operation", operation,
		"week_code", weekCode,
		"error", errorValue.Error(),
	)
}
