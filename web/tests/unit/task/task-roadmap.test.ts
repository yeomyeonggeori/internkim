import { describe, expect, test } from 'bun:test';
import { relatedTaskIDs } from '../../../src/routes/task/task-roadmap-focus';
import {
	buildTaskRoadmap,
	isTaskRoadmapWeekend,
	moveTaskDateRange,
	taskRoadmapBar,
	taskRoadmapDays,
	taskRoadmapMonthSegments
} from '../../../src/routes/task/task-roadmap-model';
import type { Task } from '../../../src/routes/task/task-types';

const november = { year: 2026, month: 11 };
const novemberDays = taskRoadmapDays(november, 0);

function task(id: string, fields: Partial<Task> = {}): Task {
	return {
		id, ownerID: 'owner', ownerName: '이샘플', participantIDs: [], participantNames: [],
		business: null, type: null, content: id, size: 'M', status: 'in_progress', weekCode: '2026-W45',
		...fields
	};
}

describe('roadmap bars', () => {
	test('a task inside the month spans its own days', () => {
		expect(taskRoadmapBar(task('a', { startDate: '2026-11-09', endDate: '2026-11-27' }), novemberDays))
			.toEqual({ kind: 'span', firstDay: 8, lastDay: 26, startsBefore: false, endsAfter: false });
	});

	test('a task crossing the month edge is cut at the edge and says so', () => {
		expect(taskRoadmapBar(task('a', { startDate: '2026-10-20', endDate: '2026-12-04' }), novemberDays))
			.toEqual({ kind: 'span', firstDay: 0, lastDay: 29, startsBefore: true, endsAfter: true });
	});

	test('a task with only one date is a single day', () => {
		expect(taskRoadmapBar(task('a', { endDate: '2026-11-06' }), novemberDays))
			.toEqual({ kind: 'span', firstDay: 5, lastDay: 5, startsBefore: false, endsAfter: false });
	});

	test('tasks outside the month point to where they are', () => {
		expect(taskRoadmapBar(task('a', { startDate: '2026-12-07', endDate: '2026-12-09' }), novemberDays)).toEqual({ kind: 'after', dateISO: '2026-12-07' });
		expect(taskRoadmapBar(task('a', { startDate: '2026-10-01', endDate: '2026-10-09' }), novemberDays)).toEqual({ kind: 'before', dateISO: '2026-10-09' });
		expect(taskRoadmapBar(task('a'), novemberDays)).toEqual({ kind: 'undated' });
	});
});

describe('roadmap groups', () => {
	test('groups follow the board status order and sort rows by start date, undated last', () => {
		const groups = buildTaskRoadmap([
			task('late', { startDate: '2026-11-20' }),
			task('undated'),
			task('early', { startDate: '2026-11-02' }),
			task('planned', { status: 'planned', startDate: '2026-11-10' })
		], novemberDays, new Map());
		expect(groups.map((group) => [group.status, group.rows.map((row) => row.task.id)])).toEqual([
			['planned', ['planned']],
			['in_progress', ['early', 'late', 'undated']]
		]);
	});

	test('completed work outside the month stays out, open work outside it stays in', () => {
		const groups = buildTaskRoadmap([
			task('old done', { status: 'completed', startDate: '2026-09-01' }),
			task('old open', { status: 'planned', startDate: '2026-09-01' })
		], novemberDays, new Map());
		expect(groups.flatMap((group) => group.rows.map((row) => row.task.id))).toEqual(['old open']);
	});

	test('only parents carry child progress', () => {
		const progress = { completed: 1, total: 3, percent: 33 };
		const [group] = buildTaskRoadmap([task('parent', { startDate: '2026-11-02' }), task('child', { startDate: '2026-11-03', parentTaskID: 'parent' })], novemberDays, new Map([['parent', progress]]));
		expect(group?.rows.map((row) => row.childProgress)).toEqual([progress, undefined]);
	});
});

describe('dragging dates', () => {
	test('moving keeps the length', () => {
		expect(moveTaskDateRange({ startDate: '2026-11-09', endDate: '2026-11-27' }, 3)).toEqual({ startDate: '2026-11-12', endDate: '2026-11-30' });
	});
});

describe('timeline', () => {
	test('one continuous run of days spans the months around the centre, cut into month segments', () => {
		const days = taskRoadmapDays(november, 1);
		expect([days[0], days.at(-1), days.length]).toEqual(['2026-10-01', '2026-12-31', 92]);
		expect(taskRoadmapMonthSegments(days).map((segment) => [segment.month.month, segment.firstDay, segment.dayCount])).toEqual([[10, 0, 31], [11, 31, 30], [12, 61, 31]]);
	});

	test('weekends are red days', () => {
		expect(['2026-11-07', '2026-11-08', '2026-11-09'].map(isTaskRoadmapWeekend)).toEqual([true, true, false]);
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
