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
		expect(Object.keys(summary.metrics.memberDistances).length).toBe(10);
		expect(summary.metrics.memberScores).toEqual(summary.metrics.memberDistances);
		expect(summary.metrics.totalScore).toBe(summary.metrics.totalDistance);
		expect(summary.tasks.length > 0).toBe(true);
		expect(summary.metrics.totalTasks).toBe(summary.tasks.length);
		expect(summary.report.weeklyDistanceTrend.currentValues.length).toBe(7);
		expect(summary.report.weeklyDistanceTrend.labels).toEqual(['1', '2', '3', '4', '5', '6', '7']);
		expect(summary.report.monthlyDistanceTrend.currentValues.length).toBe(30);
		expect(summary.metrics.completedTasks > 0).toBe(true);
		expect(summary.metrics.requestedTasks > 0).toBe(true);
		expect(summary.metrics.pausedTasks > 0).toBe(true);
		expect(summary.metrics.stoppedTasks > 0).toBe(true);
	});

	test('varies mock report data by selected week', () => {
		const previousWeek = createDevFlowSummary('26W22', 'admin@example.com');
		const currentWeek = createDevFlowSummary('26W23', 'admin@example.com');
		const nextWeek = createDevFlowSummary('26W24', 'admin@example.com');

		expect(previousWeek.week.code).toBe('26W22');
		expect(currentWeek.week.code).toBe('26W23');
		expect(nextWeek.week.code).toBe('26W24');
		expect(previousWeek.report.weeklyDistanceTrend.currentValues).not.toEqual(currentWeek.report.weeklyDistanceTrend.currentValues);
		expect(nextWeek.report.weeklyDistanceTrend.currentValues).not.toEqual(currentWeek.report.weeklyDistanceTrend.currentValues);
		expect(previousWeek.metrics.memberDistances).not.toEqual(currentWeek.metrics.memberDistances);
		expect(nextWeek.metrics.memberDistances).not.toEqual(currentWeek.metrics.memberDistances);
	});
});
