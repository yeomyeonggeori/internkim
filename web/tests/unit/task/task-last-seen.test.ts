import { describe, expect, test } from 'bun:test';
import { forgetLastSeenTask, lastSeenTask, rememberTask } from '../../../src/routes/task/task-last-seen';
import type { TaskState, TaskSummary } from '../../../src/routes/task/task-types';

const state = { tasks: [], members: [] } as unknown as TaskState;
const summary = { tasks: [], weeklyTasks: [] } as unknown as TaskSummary;

describe('what the flow screen shows before its refresh arrives', () => {
	test('nothing has been seen until a load succeeds', () => {
		forgetLastSeenTask();
		expect(lastSeenTask()).toEqual({ state: null, summary: null });
	});

	test('a finished load is what the next visit paints straight away', () => {
		rememberTask(state, summary);
		expect(lastSeenTask().state).toBe(state);
		expect(lastSeenTask().summary).toBe(summary);
	});

	test('signing out leaves nothing for the next account to open on', () => {
		rememberTask(state, summary);
		forgetLastSeenTask();
		expect(lastSeenTask().state).toBe(null);
	});
});
