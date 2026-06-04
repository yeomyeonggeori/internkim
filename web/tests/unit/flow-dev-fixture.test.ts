import { describe, expect, test } from 'bun:test';
import { createDevFlowSummary } from '../../src/routes/flow/dev-flow-fixture';

describe('createDevFlowSummary', () => {
	test('builds a populated local Flow summary for the requested week', () => {
		const summary = createDevFlowSummary('26W16', 'admin@example.com');

		expect(summary.week).toMatchObject({
			code: '26W16',
			startISO: '2026-04-13',
			endISO: '2026-04-19',
			previous: '26W15',
			next: '26W17'
		});
		expect(summary.currentUserEmail).toBe('admin@example.com');
		expect(summary.currentUserName).toBe('김철수');
		expect(summary.source).toBe('dev-mock');
		expect(summary.members.length).toBe(10);
		expect(Object.keys(summary.metrics.memberScores).length).toBe(10);
		expect(summary.tasks.length > 0).toBe(true);
		expect(summary.metrics.totalTasks).toBe(summary.tasks.length);
		expect(summary.metrics.completedTasks > 0).toBe(true);
		expect(summary.metrics.requestedTasks > 0).toBe(true);
		expect(summary.metrics.pausedTasks > 0).toBe(true);
		expect(summary.metrics.stoppedTasks > 0).toBe(true);
	});
});
