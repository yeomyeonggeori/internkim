import { describe, expect, test } from 'bun:test';
import { createClient } from '@supabase/supabase-js';
import { taskBoardWindow, taskBoardDatePredicate, taskChildProgressForBoard, type TaskCardRow } from '$lib/server/public-api/record/task-board';
import { taskPage, type TaskPageScope } from '$lib/server/public-api/record/task-page';
import { matchesTaskBoardWeek, buildTaskBoard } from '../../../../../src/routes/task/task-board-model';
import { buildTaskChildProgress } from '../../../../../src/routes/task/task-relationships';
import { taskBoard, taskList } from '$lib/server/public-api/record/task-tools';
import { taskBoardInputSchema, taskBoardResultSchema, taskListInputSchema, taskListResultSchema } from '$lib/server/public-api/catalog/tools';
import { WorkspaceTaskScope } from '$lib/server/public-api/catalog/workspace-task';
import { labelsOfVocabulary } from '$lib/server/public-api/record/labels';
import { leavesTaskLabelsUndecided } from '$lib/server/public-api/record/task-labels';
import { dayOfInstant, dayShifted, instantOfDay, weekWindow } from '$lib/server/public-api/record/days';
import { taskWeekCodeForDateISO } from '$lib/task/task-week-code';
import type { Task } from '../../../../../src/routes/task/task-types';
import type { RecordContext } from '$lib/server/public-api/record/company';
import { personInTheDirectory } from './directory-fixture';

type RecordRow = TaskCardRow & { is_whole_day?: boolean };

function row(id: string, fields: Partial<RecordRow> = {}): RecordRow {
	return {
		id, parent_task_id: null, title: id, status: 'completed', business: null, type: null,
		size: 'M', starts_at: null, ends_at: '2020-01-01T00:00:00Z', task_last_day: '2020-01-01',
		created_at: '2020-01-01T00:00:00Z', updated_at: '2020-01-01T00:00:00Z', requester_id: null,
		task_participant: [{ member_id: 'member-1' }], task_deletable: true, task_updatable: true, ...fields
	};
}

function taskOf(row: TaskCardRow, timeZone = 'UTC'): Task {
	return {
		id: row.id, parentTaskID: row.parent_task_id ?? undefined, content: row.title,
		ownerID: '', ownerName: '', participantIDs: row.task_participant.map(person => person.member_id),
		participantNames: [], business: row.business, type: row.type, size: row.size ?? '', status: row.status,
		startDate: dayOfInstant(timeZone, row.starts_at), endDate: row.task_last_day ?? '', weekCode: ''
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

function unquoted(value: string): string {
	return value.startsWith('"') ? JSON.parse(value) : value;
}

function searchTextOf(row: TaskCardRow): string {
	return [row.title, row.business, row.type, row.status].filter(Boolean).join('\ue000').toLowerCase().replace(/\s/g, '');
}

function fieldOf(field: string, row: RecordRow): string | null | undefined {
	if (field === 'task_is_work') return 'true';
	if (field === 'is_whole_day') return String(row.is_whole_day ?? false);
	if (field === 'task_search_text') return searchTextOf(row);
	if (field === 'task_sole_participant') return row.task_participant.length === 1 ? row.task_participant[0].member_id : null;
	const fields: Record<string, string | null | undefined> = {
		status: row.status, ends_at: row.ends_at, starts_at: row.starts_at, task_last_day: row.task_last_day,
		updated_at: row.updated_at, id: row.id, organization_id: row.organization_id, opportunity_id: row.opportunity_id
	};
	return fields[field];
}

function isLike(value: string, pattern: string): boolean {
	const expression = pattern.replace(/\\(.)|%|_|[.*+?^${}()|[\]]/g, (token, escaped: string | undefined) =>
		escaped !== undefined ? escaped.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') : token === '%' ? '.*' : token === '_' ? '.' : `\\${token}`);
	return new RegExp(`^${expression}$`, 's').test(value);
}

function compared(value: string, expected: string, field: string): number {
	if (field === 'id') return value < expected ? -1 : value > expected ? 1 : 0;
	return Date.parse(value) - Date.parse(expected);
}

function matches(expression: string, row: RecordRow): boolean {
	if (expression.startsWith('and(')) return clauses(expression.slice(4, -1)).every(clause => matches(clause, row));
	if (expression.startsWith('or(')) return clauses(expression.slice(3, -1)).some(clause => matches(clause, row));
	const [field, ...rest] = expression.split('.');
	const negated = rest[0] === 'not';
	const [operator, ...parts] = negated ? rest.slice(1) : rest;
	const held = matchesOperator(fieldOf(field, row), operator, unquoted(parts.join('.')), field);
	return negated ? !held : held;
}

function matchesOperator(value: string | null | undefined, operator: string, expected: string, field: string): boolean {
	if (value === undefined) throw new Error(`unhandled field ${field}`);
	if (operator === 'eq') return value !== null && (field === 'id' || field === 'status' || field.endsWith('_id') || field === 'task_is_work' ? value === expected : compared(value, expected, field) === 0);
	if (operator === 'neq') return value !== expected;
	if (operator === 'in') return expected.slice(1, -1).split(',').includes(value ?? '');
	if (operator === 'is') return expected === 'null' ? value === null : value === expected;
	if (value === null) return false;
	if (operator === 'like') return isLike(value, expected);
	if (operator === 'gte') return compared(value, expected, field) >= 0;
	if (operator === 'gt') return compared(value, expected, field) > 0;
	if (operator === 'lt') return compared(value, expected, field) < 0;
	if (operator === 'lte') return compared(value, expected, field) <= 0;
	throw new Error(`unhandled operator ${operator}`);
}

function newestFirst(left: RecordRow, right: RecordRow): number {
	if (left.updated_at !== right.updated_at) return Date.parse(right.updated_at) - Date.parse(left.updated_at);
	return left.id < right.id ? -1 : left.id > right.id ? 1 : 0;
}

function record(rows: RecordRow[], fail = false) {
	const queries: URL[] = [];
	const selectedIDs: string[][] = [];
	const fetcher: typeof fetch = Object.assign(async (input: Parameters<typeof fetch>[0], init?: RequestInit) => {
		const url = new URL(typeof input === 'string' ? input : input instanceof URL ? input.href : input.url);
		queries.push(url);
		if (fail) return Response.json({ message: 'permission denied', code: '42501' }, { status: 403 });
		let matching = rows;
		for (const predicate of url.searchParams.getAll('or')) matching = matching.filter(row => matches(`or${predicate}`, row));
		const parent = url.searchParams.get('parent_task_id');
		if (parent) {
			const parents = parent.slice(4, -1).split(',');
			matching = matching.filter(row => parents.includes(row.parent_task_id ?? ''));
		}
		const participants = url.searchParams.get('participant.member_id');
		if (participants) {
			const held = participants.slice(4, -1).split(',');
			matching = matching.filter(row => row.task_participant.some(({ member_id }) => held.includes(member_id)));
		}
		const status = url.searchParams.get('status');
		if (status) matching = matching.filter(row => matches(`status.${status}`, row));
		if (init?.method === 'HEAD') return new Response(null, { status: 200, headers: { 'content-range': `*/${matching.length}` } });
		if (url.searchParams.get('order')) matching = [...matching].sort(newestFirst);
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

async function everyPageOfTheBoard(caller: ReturnType<typeof record>['caller'], scope: TaskPageScope, limit: number) {
	const pages: Awaited<ReturnType<typeof taskPage>>[] = [];
	let cursor: string | undefined;
	do {
		const page = await taskPage(caller, scope, { limit, ...(cursor ? { cursor } : {}) }, 'task_board_get');
		pages.push(page);
		cursor = page.nextCursor;
	} while (cursor);
	return pages;
}

describe('scoped task board query', () => {
	const now = new Date('2026-10-02T12:00:00Z');
	const statuses = ['requested', 'planned', 'in_progress', 'completed', 'paused', 'rejected', 'stopped'];
	for (const timeZone of ['UTC', 'Asia/Seoul', 'America/New_York']) {
		for (const week of ['2026-09-21', '2026-09-28', '2026-10-05']) {
			test(`${timeZone} ${week}: SQL scope matches the existing board for every status/date fallback`, () => {
				const days = [null, '2026-09-20', '2026-09-28', '2026-10-04', '2026-10-05'];
				const window = taskBoardWindow(week, timeZone, now);
				const rows = statuses.flatMap(status => days.flatMap(startDay => days.flatMap(endDay => [false, true].map(isWholeDay => row(`${status}-${startDay}-${endDay}-${isWholeDay}`, {
					status, is_whole_day: isWholeDay,
					starts_at: startDay ? instantOfDay(timeZone, startDay) : null,
					ends_at: endDay ? instantOfDay(timeZone, isWholeDay ? dayShifted(endDay, 1) : endDay) : null,
					task_last_day: endDay
				})))));
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
		expect(matches(`or(${taskBoardDatePredicate(window)})`, row('last-minute', { ends_at: '2026-11-02T04:59:59Z', task_last_day: '2026-11-01' }))).toBe(true);
		expect(matches(`or(${taskBoardDatePredicate(window)})`, row('next-week', { ends_at: '2026-11-02T05:00:00Z', task_last_day: '2026-11-02' }))).toBe(false);
		expect(matches(`or(${taskBoardDatePredicate(window)})`, row('whole-last-day', { ends_at: '2026-11-02T05:00:00Z', task_last_day: '2026-11-01', is_whole_day: true }))).toBe(true);
	});

	const everyone = { everyone: true, personIDs: [] };
	const boardScope = (window = taskBoardWindow('2026-09-28', 'UTC', now)): TaskPageScope => ({
		whose: everyone, dates: taskBoardDatePredicate(window), carriesLinks: false
	});

	for (const historyCount of [100, 1501]) {
		test(`${historyCount} unrelated historical rows add nothing to a board page`, async () => {
			const { caller, queries, selectedIDs } = record([...Array.from({ length: historyCount }, (_, index) => row(`old-${index}`)), row('active', { status: 'in_progress' })]);
			const page = await taskPage(caller, boardScope(), {}, 'task_board_get');
			expect(page.items.map(row => row.id)).toEqual(['active']);
			expect(page.totalCount).toBe(1);
			expect(page.nextCursor).toBeUndefined();
			expect(queries).toHaveLength(3);
			expect(selectedIDs).toEqual([['active']]);
			expect(queries[0].searchParams.get('limit')).toBe('51');
			const selection = queries[0].searchParams.get('select') ?? '';
			for (const unused of ['note', 'location', 'organization_id', 'opportunity_id', 'notify_minutes_before']) expect(selection).not.toContain(unused);
		});
	}

	test('reads 1501 relevant board cards a page at a time, each once, newest first', async () => {
		const cards = Array.from({ length: 1501 }, (_, index) => row(`active-${String(index).padStart(4, '0')}`, {
			status: 'requested', updated_at: new Date(Date.UTC(2026, 8, 1) + Math.floor(index / 3) * 60_000).toISOString()
		}));
		const { caller, queries } = record(cards);
		const pages = await everyPageOfTheBoard(caller, boardScope(), 100);
		const ids = pages.flatMap(page => page.items.map(row => row.id));
		expect(ids).toEqual([...cards].sort(newestFirst).map(row => row.id));
		expect(pages).toHaveLength(16);
		expect(pages.every(page => page.totalCount === 1501)).toBe(true);
		expect(queries.filter(query => query.searchParams.has('limit')).every(query => query.searchParams.get('limit') === '101')).toBe(true);
	});

	test('narrows to the people asked for and counts what is unfinished across every page', async () => {
		const { caller, queries } = record([
			row('mine', { status: 'requested' }),
			row('done', { status: 'completed', ends_at: '2026-09-30T00:00:00Z', task_last_day: '2026-09-30' }),
			row('theirs', { status: 'requested', task_participant: [{ member_id: 'member-2' }] })
		]);
		const page = await taskPage(caller, { ...boardScope(), whose: { everyone: false, personIDs: ['member-1'] } }, { limit: 1 }, 'task_board_get');
		expect(page.totalCount).toBe(2);
		expect(page.unfinishedCount).toBe(1);
		expect(page.items).toHaveLength(1);
		expect(typeof page.nextCursor).toBe('string');
		expect(queries.every(query => query.searchParams.get('participant.member_id') === 'in.(member-1)')).toBe(true);
	});

	test('a query matches a field ignoring case and spacing, or the one person who holds the work', async () => {
		const { caller } = record([
			row('by-title', { status: 'requested', title: 'Quarter Report' }),
			row('by-business', { status: 'requested', business: '개 발' }),
			row('by-owner', { status: 'requested', task_participant: [{ member_id: 'member-2' }] }),
			row('shared', { status: 'requested', task_participant: [{ member_id: 'member-2' }, { member_id: 'member-1' }] })
		]);
		const found = async (text: string, soleParticipantIDs: string[] = []) =>
			(await taskPage(caller, { ...boardScope(), search: { text, soleParticipantIDs } }, {}, 'task_board_get')).items.map(row => row.id).sort();
		expect(await found('quarterreport')).toEqual(['by-title']);
		expect(await found('개발')).toEqual(['by-business']);
		expect(await found('100%')).toEqual([]);
		expect(await found('sample', ['member-2'])).toEqual(['by-owner']);
	});

	test('refuses a cursor no result gave, or one that names something other than a task position', async () => {
		const { caller } = record([]);
		for (const cursor of ['not-a-cursor', btoa('["2026-01-01T00:00:00Z"]'), btoa('["not a time","task"]'), btoa('["2026-01-01T00:00:00Z","x),or(id.gt.y"]')]) {
			await expect(taskPage(caller, boardScope(), { cursor }, 'task_board_get')).rejects.toThrow('cursor is not one a task_board_get result gave');
		}
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
		expect(queries.every(query => !query.searchParams.has('or') && query.searchParams.get('task_is_work') === 'eq.true')).toBe(true);
	});

	test('empty and denied boards cannot manufacture progress or data', async () => {
		const { caller, queries } = record([]);
		expect(await taskChildProgressForBoard(caller, [])).toEqual({});
		expect(queries).toHaveLength(0);
		const denied = record([], true);
		await expect(taskPage(denied.caller, boardScope(), {}, 'task_board_get')).rejects.toThrow('permission denied');
	});

	test('a week range answers the weeks a task is listed in, the way the board rolls work forward', async () => {
		const statuses = ['requested', 'planned', 'in_progress', 'completed', 'paused', 'rejected', 'stopped'];
		for (const timeZone of ['UTC', 'Asia/Seoul']) {
			const thisMonday = weekWindow(timeZone, now, 0, 0).from;
			const days = [null, dayShifted(thisMonday, -8), dayShifted(thisMonday, -1), thisMonday, dayShifted(thisMonday, 6), dayShifted(thisMonday, 7), dayShifted(thisMonday, 15)];
			const rows = statuses.flatMap(status => days.flatMap(startDay => days.flatMap(endDay => [false, true].map(isWholeDay => row(`${status}-${startDay}-${endDay}-${isWholeDay}`, {
				status, is_whole_day: isWholeDay, starts_at: startDay ? instantOfDay(timeZone, startDay) : null,
				ends_at: endDay ? instantOfDay(timeZone, isWholeDay ? dayShifted(endDay, 1) : endDay) : null, task_last_day: endDay
			})))));
			const weekOf = (instant: string | null) => instant ? taskWeekCodeForDateISO(dayOfInstant(timeZone, instant)) : '';
			const thisWeek = taskWeekCodeForDateISO(thisMonday);
			const listedIn = (task: TaskCardRow): string => {
				const startWeek = weekOf(task.starts_at);
				const endWeek = task.task_last_day ? taskWeekCodeForDateISO(task.task_last_day) : '';
				if (task.status === 'paused') return thisWeek;
				if (task.status === 'planned') return !task.starts_at || dayOfInstant(timeZone, task.starts_at) < dayShifted(thisMonday, 7) ? thisWeek : startWeek;
				if (['completed', 'rejected', 'stopped'].includes(task.status)) return endWeek || startWeek;
				return endWeek;
			};
			const { caller } = record(rows);
			const context: RecordContext = {
				caller, accountDirectory: caller, requesterID: 'member-1', companyID: 'company',
				people: [personInTheDirectory({ personID: 'member-1', name: 'Example', email: 'member1@example.com' })],
				labels: labelsOfVocabulary({}, timeZone), leaveKinds: [], leaveYearStart: { month: 1, day: 1 }, locale: 'en', now, decideTaskLabels: leavesTaskLabelsUndecided
			};
			for (const [weekFrom, weekTo] of [[0, 0], [-1, -1], [1, 1], [2, 2], [-2, 1]]) {
				const asked = new Set(Array.from({ length: weekTo - weekFrom + 1 }, (_, index) => taskWeekCodeForDateISO(weekWindow(timeZone, now, weekFrom + index, weekFrom + index).from)));
				const listed = await taskList(context, { scope: WorkspaceTaskScope.All, weekFrom, weekTo, limit: 100 });
				expect(listed.totalCount).toBe(rows.filter(task => asked.has(listedIn(task))).length);
				const expected = new Set(rows.filter(task => asked.has(listedIn(task))).map(task => task.id));
				expect(listed.tasks.length).toBe(Math.min(100, expected.size));
				expect(listed.tasks.every(task => expected.has(task.taskID))).toBe(true);
			}
		}
	}, 30_000);

	test('task_board preserves multi-participant scope and rejects conflicting history options', async () => {
		const { caller } = record([
			row('shared', { status: 'planned', ends_at: null, task_last_day: null, task_participant: [{ member_id: 'member-2' }, { member_id: 'member-1' }] }),
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
		expect((await taskBoard(context, { boardWeek: '2026-09-28', scope: WorkspaceTaskScope.All })).tasks).toHaveLength(2);
		expect(taskBoardInputSchema.safeParse({ boardWeek: '2026-09-28', everyWeek: true }).success).toBe(false);
		expect(taskListInputSchema.safeParse({ boardWeek: '2026-09-28' }).success).toBe(false);
		const history = await taskList(context, { everyWeek: true, scope: WorkspaceTaskScope.All });
		expect(taskListResultSchema.safeParse(history).success).toBe(true);
		expect(Object.hasOwn(history, 'boardWeek')).toBe(false);
		expect(Object.hasOwn(history, 'childProgressByParent')).toBe(false);
	});
});
