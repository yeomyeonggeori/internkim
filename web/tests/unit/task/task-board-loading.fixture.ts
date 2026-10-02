import { beforeEach, describe, expect, mock, test } from 'bun:test';
import type { RecordTaskList } from '../../../src/lib/task/task-record';

const calls: { name: string; input: Record<string, unknown> }[] = [];
let tasksGate: Promise<void> | undefined;
let directoryGate: Promise<void> | undefined;
const listed: RecordTaskList = {
	scope: 'all', count: 1,
	tasks: [{ taskID: 'parent', content: 'Sample task', status: 'in_progress', participantIDs: ['member'], weekCode: '26W40' }],
	registeredLabels: { businesses: [], types: [], sizes: [], statuses: [] },
	childProgress: [{ parentTaskID: 'parent', completed: 7, total: 9, percent: 78 }]
};

mock.module('../../../src/lib/public-api-call', () => ({
	memberAccessToken: async () => 'fixture-token',
	isRefusalCode: () => false,
	invokeTool: async (name: string, input: Record<string, unknown>) => {
		calls.push({ name, input });
		if (name === 'task_board_get' || name === 'task_list') { await tasksGate; return listed; }
		if (name === 'person_list') {
			await directoryGate;
			return { requesterID: 'member', people: [{ personID: 'member', email: 'sample@example.com', name: '이샘플' }] };
		}
		throw new Error(`Unexpected tool ${name}`);
	}
}));

const { taskState, taskBoardState, forgetTaskStateRead } = await import('../../../src/lib/task/task-state');
const { clearStoredTaskSnapshot } = await import('../../../src/routes/task/task-snapshot-storage');
const { mergeTaskSummary, taskWeeklySummaryOf } = await import('../../../src/routes/task/task-api');

beforeEach(() => { calls.length = 0; tasksGate = undefined; directoryGate = undefined; clearStoredTaskSnapshot(); });

describe('board loading', () => {
	test('starts board and directory concurrently without requesting full history', async () => {
		const tasks = Promise.withResolvers<void>();
		const people = Promise.withResolvers<void>();
		tasksGate = tasks.promise;
		directoryGate = people.promise;
		const reading = taskState('scope', '2026-09-28');
		expect(calls).toEqual([
			{ name: 'task_board_get', input: { scope: 'all', boardWeek: '2026-09-28' } },
			{ name: 'person_list', input: {} }
		]);
		tasks.resolve(); people.resolve();
		const state = await reading;
		expect(state.completeness).toBe('board');
		expect(state.boardWeek).toBe('2026-09-28');
		expect(state.metrics.memberScores).toBeUndefined();
		expect(state.metrics.totalScore).toBeUndefined();
		expect(state.childProgressByParent?.parent).toEqual({ completed: 7, total: 9, percent: 78 });
		const summary = mergeTaskSummary(state, taskWeeklySummaryOf(state, '26W40'));
		expect(summary.completeness).toBe('board');
		expect(summary.metrics.memberScoreDetails).toBeUndefined();
		expect(summary.childProgressByParent).toEqual(state.childProgressByParent);
	});

	test('coalesces the same board and renders verified viewer scope while the directory loads', async () => {
		const people = Promise.withResolvers<void>();
		directoryGate = people.promise;
		const reading = taskState('scope', '2026-09-28');
		expect(taskState('scope', '2026-09-28')).toBe(reading);
		const board = await taskBoardState('scope', { memberID: 'member', email: 'sample@example.com', name: '이샘플', isAdmin: false }, '2026-09-28');
		expect(board.members.map(member => member.id)).toEqual(['member']);
		expect(board.completeness).toBe('board');
		expect(board.peopleReady).toBe(false);
		expect(calls.map(call => call.name)).toEqual(['task_board_get', 'person_list']);
		people.resolve();
		expect((await reading).peopleReady).toBe(true);
	});

	test('a directory denial still rejects the owner read after cards were shown', async () => {
		const people = Promise.withResolvers<void>();
		directoryGate = people.promise;
		const reading = taskState('scope', '2026-09-28');
		const failure = reading.then(() => '', error => String(error));
		const board = await taskBoardState('scope', { memberID: 'member', email: 'sample@example.com', name: '이샘플', isAdmin: false }, '2026-09-28');
		expect(board.tasks).toHaveLength(1);
		expect(board.peopleReady).toBe(false);
		people.reject(new Error('Access denied'));
		expect(await failure).toContain('Access denied');
	});

	test('full history is a separate explicit read preserving the old wire contract', async () => {
		const board = taskState('scope', '2026-09-28');
		const history = taskState('scope');
		expect(history).not.toBe(board);
		expect(taskState('scope')).toBe(history);
		const state = await history;
		await board;
		expect(calls.filter(call => call.name === 'task_list')).toEqual([{ name: 'task_list', input: { scope: 'all', everyWeek: true } }]);
		expect(state.completeness).toBe('full');
		expect(state.boardWeek).toBeUndefined();
		expect(state.metrics.memberScores).toBeDefined();
	});

	test('different weeks, permissions and accounts never share an in-flight board', async () => {
		const first = taskState('company:member:member', '2026-09-28');
		const nextWeek = taskState('company:member:member', '2026-10-05');
		const admin = taskState('company:member:admin', '2026-09-28');
		const otherCompany = taskState('other:member:member', '2026-09-28');
		expect(calls.filter(call => call.name === 'task_board_get')).toHaveLength(4);
		await Promise.all([first, nextWeek, admin, otherCompany]);
	});

	test('writes and explicit refresh cannot reuse a pre-write response', async () => {
		const first = taskState('scope', '2026-09-28');
		clearStoredTaskSnapshot();
		const afterWrite = taskState('scope', '2026-09-28');
		expect(afterWrite).not.toBe(first);
		forgetTaskStateRead('scope');
		const refresh = taskState('scope', '2026-09-28');
		expect(refresh).not.toBe(afterWrite);
		await Promise.all([first, afterWrite, refresh]);
		expect(calls.filter(call => call.name === 'task_board_get')).toHaveLength(3);
	});
});
