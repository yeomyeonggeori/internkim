import { afterEach, beforeEach, describe, expect, test } from 'bun:test';
import { clearStoredTaskSnapshot, storeTaskSnapshot, storedTaskSnapshot, storeTaskBoardSnapshot, storedTaskBoardSnapshot, subscribeTaskSnapshotInvalidation } from '../../../src/routes/task/task-snapshot-storage';
import type { TaskState } from '../../../src/routes/task/task-types';

const state: TaskState = {
	completeness: 'full',
	peopleReady: true,
	tasks: [], members: [], statusOptions: [], currentUserEmail: 'member@example.com', currentUserName: 'Sample', isAdmin: false,
	metrics: { totalTasks: 0, completedTasks: 0, requestedTasks: 0, pausedTasks: 0, stoppedTasks: 0, statusCounts: {}, businessCounts: {}, typeCounts: {} },
	definitions: { categories: [], types: [], sizes: [] }
};
const scope = JSON.stringify(['plane', 'company', 'member', 'member']);
let previousWindow: PropertyDescriptor | undefined;
let values: Map<string, string>;
let storage: Storage;

beforeEach(() => {
	previousWindow = Object.getOwnPropertyDescriptor(globalThis, 'window');
	values = new Map();
	storage = { getItem: key => values.get(key) ?? null, setItem: (key, value) => { values.set(key, value); }, removeItem: key => { values.delete(key); } } as Storage;
	Object.defineProperty(globalThis, 'window', { value: { localStorage: storage }, configurable: true });
});
afterEach(() => {
	if (previousWindow) Object.defineProperty(globalThis, 'window', previousWindow);
	else Reflect.deleteProperty(globalThis, 'window');
});

describe('durable task snapshots', () => {
	test('a fetched preview without a validated directory is never persisted', () => {
		const preview: TaskState = { ...state, completeness: 'board', peopleReady: false, boardWeek: '2026-09-28' };
		storeTaskBoardSnapshot(scope, preview, 1000);
		storeTaskSnapshot(scope, { ...preview, completeness: 'full' }, 1000);
		expect(values.size).toBe(0);
	});
	test('a partial board never enters full history storage and is scoped by account and week', () => {
		const board: TaskState = { ...state, completeness: 'board', boardWeek: '2026-09-28', childProgressByParent: { parent: { completed: 3, total: 5, percent: 60 } } };
		storeTaskSnapshot(scope, board, 1000);
		expect(storedTaskSnapshot(scope, 2000)).toBeNull();
		storeTaskBoardSnapshot(scope, board, 1000);
		expect(storedTaskBoardSnapshot(scope, '2026-09-28', 2000)?.state).toEqual(board);
		expect(storedTaskBoardSnapshot(scope, '2026-10-05', 2000)).toBeNull();
		expect(storedTaskBoardSnapshot('other-scope', '2026-09-28', 2000)).toBeNull();
		expect(storedTaskSnapshot(scope, 2000)).toBeNull();
	});
	test('board snapshots are bounded and expire', () => {
		for (let index = 0; index < 8; index++) storeTaskBoardSnapshot(`scope-${index}`, { ...state, completeness: 'board', boardWeek: '2026-09-28' }, 1000 + index);
		expect(storedTaskBoardSnapshot('scope-0', '2026-09-28', 2000)).toBeNull();
		expect(storedTaskBoardSnapshot('scope-7', '2026-09-28', 2000)).not.toBeNull();
		expect(storedTaskBoardSnapshot('scope-7', '2026-09-28', 1007 + 86400001)).toBeNull();
	});
	test('a write clears both datasets and immediately notifies mounted read guards', () => {
		storeTaskSnapshot(scope, state, 1000);
		storeTaskBoardSnapshot(scope, { ...state, completeness: 'board', boardWeek: '2026-09-28' }, 1000);
		let invalidations = 0;
		const unsubscribe = subscribeTaskSnapshotInvalidation(() => { invalidations++; });
		clearStoredTaskSnapshot(scope);
		expect(invalidations).toBe(1);
		expect(storedTaskSnapshot(scope, 2000)).toBeNull();
		expect(storedTaskBoardSnapshot(scope, '2026-09-28', 2000)).toBeNull();
		unsubscribe();
		clearStoredTaskSnapshot(scope);
		expect(invalidations).toBe(1);
	});
	test('survives a new browser window and expires after one day', () => {
		storeTaskSnapshot(scope, state, 1000);
		Object.defineProperty(globalThis, 'window', { value: { localStorage: storage }, configurable: true });
		expect(storedTaskSnapshot(scope, 2000)?.state).toEqual(state);
		expect(storedTaskSnapshot(scope, 1000 + 86400001)).toBeNull();
		expect(values.size).toBe(0);
	});
	test('a different account or permission scope cannot read the previous data', () => {
		storeTaskSnapshot(scope, state, 1000);
		expect(storedTaskSnapshot(JSON.stringify(['plane', 'company', 'member', 'admin']), 2000)).toBeNull();
		expect(values.size).toBe(0);
	});
	test('rejects a changed version, invalid data and future timestamps', () => {
		for (const change of [{ version: 2 }, { state: {} }, { savedAt: 3000 }]) {
			storeTaskSnapshot(scope, state, 1000);
			const key = [...values.keys()][0];
			values.set(key, JSON.stringify({ ...JSON.parse(values.get(key)!), ...change }));
			expect(storedTaskSnapshot(scope, 2000)).toBeNull();
		}
	});
	test('storage read and quota failures leave server loading available', () => {
		Object.defineProperty(globalThis, 'window', { value: { get localStorage() { throw new Error('blocked'); } }, configurable: true });
		expect(() => storeTaskSnapshot(scope, state)).not.toThrow();
		expect(storedTaskSnapshot(scope)).toBeNull();
		expect(() => clearStoredTaskSnapshot()).not.toThrow();
	});
	test('targeted invalidation preserves another scope and logout removes this dataset', () => {
		storeTaskSnapshot(scope, state, 1000);
		clearStoredTaskSnapshot('another-scope');
		expect(storedTaskSnapshot(scope, 2000)).not.toBeNull();
		clearStoredTaskSnapshot();
		expect(values.size).toBe(0);
	});
});
