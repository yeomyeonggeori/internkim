import { describe, expect, test } from 'bun:test';
import {
	devPopupOverflowCompletedTaskTitles,
	devPopupOverflowDate,
	devPopupOverflowDisplayName
} from '../../dev-popup-overflow-fixture';
import { createDevFlowSummary } from '../../src/routes/flow/dev-flow-fixture';
import { buildDevFlowMemberScoreDetails } from '../../src/routes/flow/dev-flow-fixture-score';

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
		expect(summary.currentWeek).toMatchObject({
			code: '26W23',
			isCurrent: true
		});
		expect(summary.currentUserEmail).toBe('admin@example.com');
		expect(summary.currentUserName).toBe('김철수');
		expect(summary.source).toBe('dev-mock');
		expect(summary.members.length).toBe(10);
		expect(Object.keys(summary.metrics.memberDistances).length).toBe(10);
		expect(summary.metrics.memberScores).not.toEqual(summary.metrics.memberDistances);
		expect(summary.metrics.memberScores).toMatchObject({
			designer: 100,
			engineer: 103,
			qa: 101
		});
		expect(summary.metrics.memberScoreDetails.qa).toMatchObject({
			currentScore: 101
		});
		expect(summary.metrics.totalScore).toBe(603);
		expect(summary.members.find((member) => member.name === '윤도현')?.score).toBe(101);
		expect(summary.tasks.length > 0).toBe(true);
		expect((summary.weeklyTasks?.length ?? 0) > 0).toBe(true);
		expect(summary.tasks.length > (summary.weeklyTasks?.length ?? 0)).toBe(true);
		expect(summary.metrics.totalTasks).toBe(summary.weeklyTasks?.length);
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
		expect(previousWeek.tasks.map((task) => task.id)).toEqual(currentWeek.tasks.map((task) => task.id));
		expect(nextWeek.tasks.map((task) => task.id)).toEqual(currentWeek.tasks.map((task) => task.id));
		expect(previousWeek.weeklyTasks?.every((task) => task.weekCode === '26W22')).toBe(true);
		expect(currentWeek.weeklyTasks?.every((task) => task.weekCode === '26W23')).toBe(true);
		expect(nextWeek.weeklyTasks?.every((task) => task.weekCode === '26W24')).toBe(true);
	});

	test('includes four completed June popup overflow tasks for 김철수', () => {
		const summary = createDevFlowSummary('26W25', 'admin@example.com');
		const tasks = summary.tasks
			.filter((task) => task.ownerName === devPopupOverflowDisplayName && task.status === '완료' && task.endDate === devPopupOverflowDate)
			.map((task) => task.content);

		expect(tasks).toEqual(devPopupOverflowCompletedTaskTitles);
	});

	test('scores member growth against the recent five period baseline', () => {
		const details = buildDevFlowMemberScoreDetails(
			[
				scoreTask('current', 'D4', '2026-06-01'),
				scoreTask('previous-1', 'D3', '2026-05-25'),
				scoreTask('previous-2', 'D2', '2026-05-18'),
				scoreTask('previous-3', 'D1', '2026-05-11')
			],
			[
				{
					id: 'member-a',
					name: '김철수',
					email: 'member-a@example.com',
					role: 'member',
					mattermostStatus: '',
					activeTaskCount: 0,
					completeTaskCount: 0
				}
			],
			{
				categories: ['여명거리'],
				types: ['기능'],
				sizes: [
					sizeDefinition('D1', 1),
					sizeDefinition('D2', 2),
					sizeDefinition('D3', 3),
					sizeDefinition('D4', 4)
				]
			},
			{
				code: '26W23',
				startISO: '2026-06-01',
				endISO: '2026-06-07',
				previous: '26W22',
				next: '26W24',
				isCurrent: true
			}
		);

		expect(details['member-a']?.weeklyScore).toBe(108);
	});
});

function scoreTask(id: string, size: string, endDate: string) {
	return {
		id,
		ownerID: 'member-a',
		ownerName: '김철수',
		participantIDs: ['member-a'],
		participantNames: ['김철수'],
		business: '여명거리',
		type: '기능',
		content: id,
		goal: '',
		size,
		status: '완료',
		statusRank: 0,
		startDate: endDate,
		endDate,
		weekCode: '',
		flag: 0
	};
}

function sizeDefinition(name: string, distanceKm: number) {
	return {
		name,
		distanceKm,
		maxHours: distanceKm,
		developmentExample: '',
		otherExample: '',
		note: '',
		score: distanceKm,
		label: `${distanceKm}km`
	};
}
