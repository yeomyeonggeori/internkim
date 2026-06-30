import type { TaskRunsResponse, TaskRunSummary } from '../../src/routes/tasks/tasks-api';

const taskRunFixtureCount = 60;
const taskRunFixturePageSize = 15;
const taskRunFixtureBaseTime = Date.UTC(2026, 5, 17, 1, 0, 0);
const taskRunFixtureIntervalMilliseconds = 12 * 60 * 1000;

export function createTaskRunsFixture(): TaskRunsResponse {
	const taskRuns = Array.from({ length: taskRunFixtureCount }, (_, index) => createTaskRunSummary(index));
	return {
		taskRuns: taskRuns.slice(0, taskRunFixturePageSize),
		totalCount: taskRuns.length,
		dailyCostSummaries: [],
		dailyCostScope: {
			taskRunLimit: 500,
			taskRunCount: taskRuns.length,
			totalTaskRunCount: taskRuns.length,
			isTruncated: false
		}
	};
}

function createTaskRunSummary(index: number): TaskRunSummary {
	const occurredAt = new Date(taskRunFixtureBaseTime - index * taskRunFixtureIntervalMilliseconds).toISOString();
	const status = index % 4 === 0 ? 'failed' : 'completed';
	return {
		taskRunID: `dev-task-run-${String(index + 1).padStart(3, '0')}`,
		requesterPersonID: 'dev-person-admin',
		status,
		prompt: `개발 확인용 작업 기록 ${index + 1}`,
		failureReason: status === 'failed' ? 'mock failure reason for pagination check' : undefined,
		llmCostUSD: 0.0012 + index * 0.0001,
		llmCallCount: index + 1,
		createdAt: occurredAt,
		updatedAt: occurredAt
	};
}
