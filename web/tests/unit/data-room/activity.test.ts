import { describe, expect, test } from 'bun:test';
import {
	activityTasks,
	contributionTotal,
	contributionWeeks,
	redactedSegments,
	type CompanyActivity
} from '../../../src/lib/data-room/activity';

const activity = (contributions: CompanyActivity['contributions']): CompanyActivity => ({
	today: '2026-10-16',
	attendance: { date: '2026-10-16', summary: { memberCount: 0, working: 0, done: 0, away: 0, notStarted: 0, needsCheckout: 0 }, teams: [] },
	board: {
		vocabulary: {},
		counts: { in_progress: 1 },
		tasks: [{ id: 't1', title: '투자자 •• ••', status: 'in_progress', people: ['p2', 'p1'] }]
	},
	contributions,
	people: { p1: { name: '이*플' }, p2: { name: '박*시', image: 'data:image/png;base64,AA' } }
});

describe('the company at work', () => {
	test('lays a year out as weeks starting on Monday and leaves the days after today empty', () => {
		const weeks = contributionWeeks(activity([{ date: '2026-10-16', count: 2 }]), 53);
		expect(weeks).toHaveLength(53);
		const lastWeek = weeks.at(-1)?.days ?? [];
		expect(lastWeek[0]?.date).toBe('2026-10-12');
		expect(lastWeek[4]).toEqual({ date: '2026-10-16', count: 2, level: 4 });
		expect(lastWeek.slice(5)).toEqual([null, null]);
	});

	test('shades a day by where it falls among the busy days, so one outlier does not wash out the rest', () => {
		const days = [1, 2, 3, 4, 40].map((count, index) => ({ date: `2026-10-1${index + 2}`, count }));
		const levels = contributionWeeks(activity(days), 2).flatMap((week) => week.days)
			.filter((day) => day !== null && day.count > 0).map((day) => day?.level);
		expect(levels).toEqual([1, 2, 3, 4, 4]);
		expect(contributionTotal(activity(days))).toBe(50);
	});

	test('keeps what a masked text shows and turns each hidden run into a bar, a little uneven but the same for the same seed', () => {
		const segments = redactedSegments('t1', '김•• 외 ••• Kim');
		expect(segments.map((segment) => ('text' in segment ? segment.text : segment.redactedLength))).toEqual(['김', 2, ' 외 ', 3, ' Kim']);
		expect(segments.every((segment) => 'text' in segment || (segment.widthScale >= 0.92 && segment.widthScale <= 1.08))).toBe(true);
		expect(redactedSegments('t1', '김•• 외 ••• Kim')).toEqual(segments);
	});

	test('turns the board into tasks the task board card draws, named only by masked names', () => {
		const [task] = activityTasks(activity([]));
		expect(task.participantNames).toEqual(['박*시', '이*플']);
		expect(task.ownerID).toBe('p2');
		expect(task.content).toBe('투자자 •• ••');
	});
});
