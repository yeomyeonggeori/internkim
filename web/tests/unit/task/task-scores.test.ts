import { describe, expect, test } from 'bun:test';
import {
	currentScoresOf,
	memberScoreDetails,
	memberTaskTallies,
	startOfISOWeek,
	totalScoreOf
} from '../../../src/lib/task/task-scores';
import type { Task } from '../../../src/routes/task/task-types';

const weekStart = new Date('2026-08-10T00:00:00Z');

function completedTask(memberIDs: string[], endDate: string, size: string): Task {
	return {
		id: `${endDate}-${size}-${memberIDs.join('-')}`,
		ownerID: memberIDs[0] ?? '',
		ownerName: '',
		participantIDs: memberIDs,
		participantNames: [],
		business: '',
		type: '',
		content: '완료된 업무',
		size,
		status: 'completed',
		endDate,
		weekCode: ''
	};
}

describe('member score details', () => {
	test('scores a steady five-week pace at exactly the baseline', () => {
		const tasks = ['2026-08-10', '2026-08-03', '2026-07-27', '2026-07-20', '2026-07-13'].map((endDate) =>
			completedTask(['sample-member'], endDate, 'M')
		);

		const details = memberScoreDetails(tasks, ['sample-member'], weekStart);

		expect(details['sample-member'].weeklyScore).toBe(100);
		expect(details['sample-member'].monthlyScore).toBe(111);
		expect(details['sample-member'].currentScore).toBe(105);
	});

	test('weights the most recent period highest', () => {
		const details = memberScoreDetails(
			[completedTask(['sample-member'], '2026-08-10', 'M')],
			['sample-member'],
			weekStart
		);

		expect(details['sample-member'].weeklyScore).toBe(115);
	});

	test('gives every participant the whole distance of a shared task', () => {
		const details = memberScoreDetails(
			[completedTask(['sample-member', 'other-member'], '2026-08-10', 'M')],
			['sample-member', 'other-member'],
			weekStart
		);

		expect(details['sample-member'].weeklyScore).toBe(115);
		expect(details['other-member'].weeklyScore).toBe(115);
	});

	test('scores a member with no completed work at zero', () => {
		const details = memberScoreDetails([], ['sample-member'], weekStart);

		expect(details['sample-member']).toEqual({ weeklyScore: 0, monthlyScore: 0, currentScore: 0 });
	});

	test('ignores work that is not complete', () => {
		const planned = { ...completedTask(['sample-member'], '2026-08-10', 'M'), status: 'in_progress' };

		const details = memberScoreDetails([planned], ['sample-member'], weekStart);

		expect(details['sample-member'].currentScore).toBe(0);
	});

	test('ignores work completed before the five periods being scored', () => {
		const details = memberScoreDetails(
			[completedTask(['sample-member'], '2025-01-06', 'XXL')],
			['sample-member'],
			weekStart
		);

		expect(details['sample-member'].currentScore).toBe(0);
	});

	test('ignores a size no definition names', () => {
		const details = memberScoreDetails(
			[completedTask(['sample-member'], '2026-08-10', 'HUGE')],
			['sample-member'],
			weekStart
		);

		expect(details['sample-member'].currentScore).toBe(0);
	});
});

describe('member task tallies', () => {
	test('counts distance only for completed work', () => {
		const tasks = [
			completedTask(['sample-member'], '2026-08-10', 'M'),
			{ ...completedTask(['sample-member'], '2026-08-10', 'XL'), status: 'in_progress' }
		];

		const tallies = memberTaskTallies(tasks, ['sample-member']);

		expect(tallies['sample-member'].distance).toBe(3);
		expect(tallies['sample-member'].completeTaskCount).toBe(1);
		expect(tallies['sample-member'].activeTaskCount).toBe(1);
	});

	test('leaves rejected and stopped work out of the active count', () => {
		const tasks = ['rejected', 'stopped', 'paused', 'planned'].map((status) => ({
			...completedTask(['sample-member'], '2026-08-10', 'M'),
			id: status,
			status
		}));

		const tallies = memberTaskTallies(tasks, ['sample-member']);

		expect(tallies['sample-member'].activeTaskCount).toBe(2);
		expect(tallies['sample-member'].completeTaskCount).toBe(0);
	});

	test('counts a task once for a member listed twice', () => {
		const tasks = [completedTask(['sample-member', 'sample-member'], '2026-08-10', 'M')];

		const tallies = memberTaskTallies(tasks, ['sample-member']);

		expect(tallies['sample-member'].completeTaskCount).toBe(1);
		expect(tallies['sample-member'].distance).toBe(3);
	});
});

describe('score periods', () => {
	test('counts the fifth week back but not the sixth', () => {
		const fifth = memberScoreDetails(
			[completedTask(['sample-member'], '2026-07-13', 'M')],
			['sample-member'],
			weekStart
		);
		const sixth = memberScoreDetails(
			[completedTask(['sample-member'], '2026-07-06', 'M')],
			['sample-member'],
			weekStart
		);

		expect(fifth['sample-member'].weeklyScore).toBe(85);
		expect(sixth['sample-member'].weeklyScore).toBe(0);
	});

	test('counts work dated later this month toward the month but not the week', () => {
		const details = memberScoreDetails(
			[completedTask(['sample-member'], '2026-08-17', 'M')],
			['sample-member'],
			weekStart
		);

		expect(details['sample-member'].weeklyScore).toBe(0);
		expect(details['sample-member'].monthlyScore).toBe(115);
	});

	test('leaves work dated in a later month out of every period', () => {
		const details = memberScoreDetails(
			[completedTask(['sample-member'], '2026-09-07', 'M')],
			['sample-member'],
			weekStart
		);

		expect(details['sample-member'].currentScore).toBe(0);
	});
});

describe('score totals', () => {
	test('sums the current score of every member', () => {
		const details = {
			'sample-member': { weeklyScore: 100, monthlyScore: 120, currentScore: 110 },
			'other-member': { weeklyScore: 80, monthlyScore: 60, currentScore: 70 }
		};

		expect(currentScoresOf(details)).toEqual({ 'sample-member': 110, 'other-member': 70 });
		expect(totalScoreOf(currentScoresOf(details))).toBe(180);
	});
});

describe('ISO week start', () => {
	test('walks back to Monday from any day of the week', () => {
		expect(startOfISOWeek(new Date('2026-08-16T23:00:00Z')).toISOString().slice(0, 10)).toBe('2026-08-10');
		expect(startOfISOWeek(new Date('2026-08-10T00:00:00Z')).toISOString().slice(0, 10)).toBe('2026-08-10');
	});
});
