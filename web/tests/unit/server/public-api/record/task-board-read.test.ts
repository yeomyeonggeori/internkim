import { describe, expect, test } from 'bun:test';
import { createClient } from '@supabase/supabase-js';
import { taskBoardWindow, taskBoardDatePredicate, tasksForBoard, taskChildProgressForBoard, type TaskCardRow } from '$lib/server/public-api/record/task-board';
import { matchesTaskBoardWeek, buildTaskBoard } from '../../../../../src/routes/task/task-board-model';
import { buildTaskChildProgress } from '../../../../../src/routes/task/task-relationships';
import { taskBoard, taskList } from '$lib/server/public-api/record/task-tools';
import { taskBoardResultSchema, taskListResultSchema } from '$lib/server/public-api/catalog/tools';
import { labelsOfVocabulary } from '$lib/server/public-api/record/labels';
import { leavesTaskLabelsUndecided } from '$lib/server/public-api/record/task-labels';
import { dayOfInstant, instantOfDay } from '$lib/server/public-api/record/days';
import type { Task } from '../../../../../src/routes/task/task-types';
import type { RecordContext } from '$lib/server/public-api/record/company';
import { personInTheDirectory } from './directory-fixture';

function row(id: string, fields: Partial<TaskCardRow> = {}): TaskCardRow {
	return {
		id, parent_task_id: null, title: id, status: 'completed', business: null, type: null,
		size: 'M', starts_at: null, ends_at: '2020-01-01T00:00:00Z',
		created_at: '2020-01-01T00:00:00Z', updated_at: '2020-01-01T00:00:00Z', requester_id: null,
		task_participant: [{ member_id: 'member-1' }], ...fields
	};
}

function taskOf(row: TaskCardRow, timeZone = 'UTC'): Task {
	return {
		id: row.id, parentTaskID: row.parent_task_id ?? undefined, content: row.title,
		ownerID: '', ownerName: '', participantIDs: row.task_participant.map(person => person.member_id),
		participantNames: [], business: row.business, type: row.type, size: row.size ?? '', status: row.status,
		startDate: dayOfInstant(timeZone, row.starts_at), endDate: dayOfInstant(timeZone, row.ends_at), weekCode: ''
	};
}

// Interpret the emitted PostgREST boolean grammar in the controlled fake record.
// This checks SQL request scope separately from the reader's final shared filter.
function clauses(value: string): string[] {
	const result: string[] = [];
	let depth = 0;
	let start = 0;
	for (let index = 0; index < value.length; index += 1) {
		if (value[index] === '(') depth += 1;
		if (value[index] === ')') depth -= 1;
		if (value[index] === ',' && depth === 0) { result.push(value.slice(start, index)); start = index + 1; }
	}
	return [...result, value.slice(start)];
}

function matches(expression: string, row: TaskCardRow): boolean {
	if (expression.startsWith('and(')) return clauses(expression.slice(4, -1)).every(clause => matches(clause, row));
	if (expression.startsWith('or(')) return clauses(expression.slice(3, -1)).some(clause => matches(clause, row));
	const [field, operator, ...parts] = expression.split('.');
	const expected = parts.join('.');
	const value = field === 'status' ? row.status : field === 'ends_at' ? row.ends_at : field === 'starts_at' ? row.starts_at : undefined;
	if (value === undefined) throw new Error(`unhandled field ${field}`);
	if (operator === 'eq') return value === expected;
	if (operator === 'in') return expected.slice(1, -1).split(',').includes(value ?? '');
	if (operator === 'is') return value === null && expected === 'null';
	if (value === null) return false;
	if (operator === 'gte') return Date.parse(value) >= Date.parse(expected);
	if (operator === 'lt') return Date.parse(value) < Date.parse(expected);
	throw new Error(`unhandled operator ${operator}`);
}

function record(rows: TaskCardRow[], fail = false) {
	const queries: URL[] = [];
	const selectedIDs: string[][] = [];
	const fetcher: typeof fetch = Object.assign(async (input: Parameters<typeof fetch>[0]) => {
		const url = new URL(typeof input === 'string' ? input : input instanceof URL ? input.href : input.url);
		queries.push(url);
		if (fail) return Response.json({ message: 'permission denied', code: '42501' }, { status: 403 });
		let matching = rows;
		const predicate = url.searchParams.get('or');
		if (predicate) matching = matching.filter(row => matches(`or${predicate}`, row));
		const parent = url.searchParams.get('parent_task_id');
		if (parent) {
			const parents = parent.slice(4, -1).split(',');
			matching = matching.filter(row => parents.includes(row.parent_task_id ?? ''));
		}
		if (url.searchParams.get('status') === 'neq.stopped') matching = matching.filter(row => row.status !== 'stopped');
		const from = Number(url.searchParams.get('offset') ?? 0);
		const limit = Number(url.searchParams.get('limit') ?? 1000);
		const page = matching.slice(from, from + limit);
		selectedIDs.push(page.map(row => row.id));
		return Response.json(page);
	}, { preconnect() {} });
	return {
		queries, selectedIDs,
		caller: createClient('https://record.example.com', 'test-key', {
			auth: { persistSession: false, autoRefreshToken: false }, global: { fetch: fetcher }
		})
	};
}

describe('scoped task board query', () => {
	const now = new Date('2026-10-02T12:00:00Z');
	const statuses = ['requested', 'planned', 'in_progress', 'completed', 'paused', 'rejected', 'stopped'];
	for (const timeZone of ['UTC', 'Asia/Seoul', 'America/New_York']) {
		for (const week of ['2026-09-21', '2026-09-28', '2026-10-05']) {
			test(`${timeZone} ${week}: SQL scope matches the existing board for every status/date fallback`, () => {
				const dates = [null, ...['2026-09-20', '2026-09-28', '2026-10-04', '2026-10-05'].map(day => instantOfDay(timeZone, day))];
				const window = taskBoardWindow(week, timeZone, now);
				const rows = statuses.flatMap(status => dates.flatMap(starts_at => dates.map(ends_at => row(`${status}-${starts_at}-${ends_at}`, { status, starts_at, ends_at }))));
				const expected = rows.filter(row => matchesTaskBoardWeek(taskOf(row, timeZone), window.options)).map(row => row.id).sort();
				const requested = rows.filter(row => matches(`or(${taskBoardDatePredicate(window)})`, row)).map(row => row.id).sort();
				expect(requested).toEqual(expected);
				expect(buildTaskBoard(rows.map(row => taskOf(row, timeZone)), window.options).flatMap(column => column.tasks.map(task => task.id)).sort()).toEqual(expected);
			});
		}
	}

	test('validates Monday dates rather than silently treating arbitrary dates as weeks', () => {
		for (const date of ['2026-10-02', '2026-02-30', '2026-13-01']) expect(() => taskBoardWindow(date, 'UTC', now)).toThrow();
	});

	test('current-week identity agrees with the existing UTC board across company midnight', () => {
		const boundary = new Date('2026-10-04T23:30:00Z');
		expect(taskBoardWindow('2026-09-28', 'Asia/Seoul', boundary).position).toBe('current');
		expect(taskBoardWindow('2026-10-05', 'Asia/Seoul', boundary).position).toBe('future');
	});

	test('a board week crossing daylight saving ends at company midnight after the offset change', () => {
		const window = taskBoardWindow('2026-10-26', 'America/New_York', new Date('2026-10-30T12:00:00Z'));
		expect(window.from).toBe('2026-10-26T04:00:00.000Z');
		expect(window.until).toBe('2026-11-02T05:00:00.000Z');
		expect(matches(`or(${taskBoardDatePredicate(window)})`, row('last-minute', { ends_at: '2026-11-02T04:59:59Z' }))).toBe(true);
		expect(matches(`or(${taskBoardDatePredicate(window)})`, row('next-week', { ends_at: '2026-11-02T05:00:00Z' }))).toBe(false);
	});

	for (const historyCount of [100, 1501]) {
		test(`${historyCount} unrelated historical rows do not add a board page`, async () => {
			const { caller, queries, selectedIDs } = record([...Array.from({ length: historyCount }, (_, index) => row(`old-${index}`)), row('active', { status: 'in_progress' })]);
			expect((await tasksForBoard(caller, taskBoardWindow('2026-09-28', 'UTC', now))).map(row => row.id)).toEqual(['active']);
			expect(queries).toHaveLength(1);
			expect(selectedIDs).toEqual([['active']]);
			const selection = queries[0].searchParams.get('select') ?? '';
			for (const unused of ['note', 'location', 'organization_id', 'opportunity_id', 'notify_minutes_before']) expect(selection).not.toContain(unused);
		});
	}

	test('never truncates 1501 relevant board cards', async () => {
		const { caller, queries } = record(Array.from({ length: 1501 }, (_, index) => row(`active-${index}`, { status: 'requested' })));
		expect(await tasksForBoard(caller, taskBoardWindow('2026-09-28', 'UTC', now))).toHaveLength(1501);
		expect(queries).toHaveLength(4);
	});

	test('direct child progress crosses weeks and participants, includes rejected, and excludes only stopped', async () => {
		const children = [
			...Array.from({ length: 501 }, (_, index) => row(`child-${index}`, { parent_task_id: 'parent', task_participant: [{ member_id: 'member-2' }] })),
			row('rejected', { parent_task_id: 'parent', status: 'rejected' }),
			row('stopped', { parent_task_id: 'parent', status: 'stopped' }),
			row('grandchild', { parent_task_id: 'child-1' }),
			row('unrelated', { parent_task_id: 'another-parent' })
		];
		const { caller, queries } = record(children);
		const progress = await taskChildProgressForBoard(caller, ['parent']);
		const expectedProgress = buildTaskChildProgress('parent', children.map(row => taskOf(row)));
		if (!expectedProgress) throw new Error('fixture must have direct children');
		expect(progress.parent).toEqual(expectedProgress);
		expect(progress.parent).toEqual({ completed: 501, total: 502, percent: 100 });
		expect(queries).toHaveLength(2);
		expect(queries.every(query => !query.searchParams.has('or'))).toBe(true);
	});

	test('empty and denied boards cannot manufacture progress or data', async () => {
		const { caller, queries } = record([]);
		expect(await taskChildProgressForBoard(caller, [])).toEqual({});
		expect(queries).toHaveLength(0);
		const denied = record([], true);
		await expect(tasksForBoard(denied.caller, taskBoardWindow('2026-09-28', 'UTC', now))).rejects.toThrow('permission denied');
	});

	test('task_board preserves multi-participant scope and rejects conflicting history options', async () => {
		const { caller } = record([
			row('shared', { status: 'planned', ends_at: null, task_participant: [{ member_id: 'member-2' }, { member_id: 'member-1' }] }),
			row('other', { status: 'requested', task_participant: [{ member_id: 'member-2' }] })
		]);
		const context: RecordContext = {
			caller, accountDirectory: caller, requesterID: 'member-1', companyID: 'company',
			people: [personInTheDirectory({ personID: 'member-1', name: 'Example', email: 'member1@example.com' }), personInTheDirectory({ personID: 'member-2', name: 'Sample', email: 'member2@example.com' })],
			labels: labelsOfVocabulary({}, 'UTC'), leaveKinds: [], leaveYearStart: { month: 1, day: 1 }, locale: 'en', now, decideTaskLabels: leavesTaskLabelsUndecided
		};
		const mine = await taskBoard(context, { boardWeek: '2026-09-28' });
		expect(taskBoardResultSchema.safeParse(mine).success).toBe(true);
		expect(mine.tasks.map(task => task.taskID)).toEqual(['shared']);
		expect(mine.tasks[0].participantIDs).toEqual(['member-2', 'member-1']);
		expect((await taskBoard(context, { boardWeek: '2026-09-28', scope: 'all' })).tasks).toHaveLength(2);
		await expect(taskBoard(context, { boardWeek: '2026-09-28', everyWeek: true })).rejects.toThrow('cannot be combined');
		await expect(taskList(context, { boardWeek: '2026-09-28' })).rejects.toThrow('use task_board');
		const history = await taskList(context, { everyWeek: true, scope: 'all' });
		expect(taskListResultSchema.safeParse(history).success).toBe(true);
		expect(Object.hasOwn(history, 'boardWeek')).toBe(false);
		expect(Object.hasOwn(history, 'childProgressByParent')).toBe(false);
	});
});
