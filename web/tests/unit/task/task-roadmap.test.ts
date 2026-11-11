import { describe, expect, test } from 'bun:test';
import { relatedTaskIDs } from '../../../src/routes/task/task-roadmap-focus';
import {
	buildTaskRoadmapRows,
	clampTaskRoadmapMove,
	formatTaskRoadmapDayNumber,
	formatTaskRoadmapPeriod,
	isTaskRoadmapWeekend,
	moveTaskDateRange,
	taskRoadmapBar,
	taskRoadmapDays
} from '../../../src/routes/task/task-roadmap-model';
import {
	isTaskRoadmapStartAllowed,
	taskRoadmapMonthStart,
	taskRoadmapShownMonth,
	taskRoadmapWeekStart
} from '../../../src/routes/task/task-roadmap-window';
import type { Task } from '../../../src/routes/task/task-types';

const twoWeeks = taskRoadmapDays('2026-11-08', 14);

function task(id: string, fields: Partial<Task> = {}): Task {
	return {
		id, ownerID: 'owner', ownerName: '이샘플', participantIDs: [], participantNames: [],
		business: null, type: null, content: id, size: 'M', status: 'in_progress', weekCode: '2026-W45',
		...fields
	};
}

describe('roadmap bars', () => {
	test('a task inside the window spans its own days', () => {
		expect(taskRoadmapBar(task('a', { startDate: '2026-11-09', endDate: '2026-11-13' }), twoWeeks))
			.toEqual({ kind: 'span', startDate: '2026-11-09', endDate: '2026-11-13', firstDay: 1, lastDay: 5, startsBefore: false, endsAfter: false });
	});

	test('a task crossing both window edges is cut at the edges and keeps its own dates', () => {
		expect(taskRoadmapBar(task('a', { startDate: '2026-10-26', endDate: '2026-12-04' }), twoWeeks))
			.toEqual({ kind: 'span', startDate: '2026-10-26', endDate: '2026-12-04', firstDay: 0, lastDay: 13, startsBefore: true, endsAfter: true });
	});

	test('a task with only one date is a single day', () => {
		expect(taskRoadmapBar(task('a', { endDate: '2026-11-12' }), twoWeeks))
			.toEqual({ kind: 'span', startDate: '2026-11-12', endDate: '2026-11-12', firstDay: 4, lastDay: 4, startsBefore: false, endsAfter: false });
	});

	test('tasks outside the window carry both of their dates', () => {
		expect(taskRoadmapBar(task('a', { startDate: '2026-11-23', endDate: '2026-11-26' }), twoWeeks)).toEqual({ kind: 'after', startDate: '2026-11-23', endDate: '2026-11-26' });
		expect(taskRoadmapBar(task('a', { startDate: '2026-11-03', endDate: '2026-11-07' }), twoWeeks)).toEqual({ kind: 'before', startDate: '2026-11-03', endDate: '2026-11-07' });
		expect(taskRoadmapBar(task('a'), twoWeeks)).toEqual({ kind: 'undated' });
	});
});

describe('roadmap rows', () => {
	test('rows form one list sorted by start date, undated last, whatever their status', () => {
		const rows = buildTaskRoadmapRows([
			task('late', { startDate: '2026-11-20' }),
			task('undated'),
			task('early', { startDate: '2026-11-02' }),
			task('planned', { status: 'planned', startDate: '2026-11-10' })
		], twoWeeks, new Map());
		expect(rows.map((row) => row.task.id)).toEqual(['early', 'planned', 'late', 'undated']);
	});

	test('completed work outside the window stays out, open work outside it stays in', () => {
		const rows = buildTaskRoadmapRows([
			task('old done', { status: 'completed', startDate: '2026-11-01' }),
			task('old open', { status: 'planned', startDate: '2026-11-01' })
		], twoWeeks, new Map());
		expect(rows.map((row) => row.task.id)).toEqual(['old open']);
	});

	test('only parents carry child progress', () => {
		const progress = { completed: 1, total: 3, percent: 33 };
		const rows = buildTaskRoadmapRows([task('parent', { startDate: '2026-11-09' }), task('child', { startDate: '2026-11-10', parentTaskID: 'parent' })], twoWeeks, new Map([['parent', progress]]));
		expect(rows.map((row) => row.childProgress)).toEqual([progress, undefined]);
	});
});

describe('dragging dates', () => {
	test('moving keeps the length', () => {
		expect(moveTaskDateRange({ startDate: '2026-11-09', endDate: '2026-11-27' }, 3)).toEqual({ startDate: '2026-11-12', endDate: '2026-11-30' });
	});

	test('a drag never carries the bar wholly out of the window', () => {
		const range = { startDate: '2026-11-10', endDate: '2026-11-12' };
		expect(clampTaskRoadmapMove(range, twoWeeks, 3)).toBe(3);
		expect(clampTaskRoadmapMove(range, twoWeeks, 30)).toBe(11);
		expect(clampTaskRoadmapMove(range, twoWeeks, -30)).toBe(-4);
	});
});

describe('visible window', () => {
	test('the window opens on the Sunday of the week holding the date', () => {
		expect(['2026-11-08', '2026-11-11', '2026-11-14'].map(taskRoadmapWeekStart)).toEqual(['2026-11-08', '2026-11-08', '2026-11-08']);
	});

	test('the window runs a given number of days from its first day', () => {
		const days = taskRoadmapDays('2026-11-29', 7);
		expect([days[0], days.at(-1), days.length]).toEqual(['2026-11-29', '2026-12-05', 7]);
	});

	test('the shown month is the month of the fourth day', () => {
		expect(taskRoadmapShownMonth('2026-11-29')).toEqual({ year: 2026, month: 12 });
		expect(taskRoadmapShownMonth('2026-11-27')).toEqual({ year: 2026, month: 11 });
	});

	test('choosing a month opens the week of its first day, unless that week shows the month before', () => {
		expect(taskRoadmapMonthStart({ year: 2026, month: 12 }, '2026-11-11')).toBe('2026-11-29');
		expect(taskRoadmapMonthStart({ year: 2027, month: 1 }, '2026-11-11')).toBe('2027-01-03');
	});

	test('choosing this month opens the week of today', () => {
		expect(taskRoadmapMonthStart({ year: 2026, month: 11 }, '2026-11-11')).toBe('2026-11-08');
	});

	test('the window moves at most six months from this month', () => {
		expect(isTaskRoadmapStartAllowed('2027-05-02', '2026-11-11')).toBe(true);
		expect(isTaskRoadmapStartAllowed('2027-06-06', '2026-11-11')).toBe(false);
		expect(isTaskRoadmapStartAllowed('2026-05-03', '2026-11-11')).toBe(true);
		expect(isTaskRoadmapStartAllowed('2026-04-26', '2026-11-11')).toBe(false);
	});

	test('weekends are red days', () => {
		expect(['2026-11-07', '2026-11-08', '2026-11-09'].map(isTaskRoadmapWeekend)).toEqual([true, true, false]);
	});

	test('the first of a month names its month, and a period of one day is one date', () => {
		expect(['2026-12-01', '2026-12-02'].map(formatTaskRoadmapDayNumber)).toEqual(['12/1', '2']);
		expect(formatTaskRoadmapPeriod({ startDate: '2026-11-03', endDate: '2026-11-07' })).toBe('11/3 – 11/7');
		expect(formatTaskRoadmapPeriod({ startDate: '2026-11-30', endDate: '2026-11-30' })).toBe('11/30');
	});
});

describe('related tasks', () => {
	const tasks = [
		task('parent'), task('child', { parentTaskID: 'parent' }), task('sibling', { parentTaskID: 'parent' }),
		task('grandchild', { parentTaskID: 'child' }), task('stranger')
	];

	test('a task in the middle brings its parent, siblings and children', () => {
		expect([...relatedTaskIDs('child', tasks)].toSorted()).toEqual(['child', 'grandchild', 'parent', 'sibling']);
	});

	test('a task with no relations stands alone', () => {
		expect([...relatedTaskIDs('stranger', tasks)]).toEqual(['stranger']);
	});
});
