import { describe, expect, test } from 'bun:test';
import { forgetLastSeenTask, lastSeenTask, rememberTask } from '../../../src/routes/task/task-last-seen';
import type { TaskState, TaskSummary } from '../../../src/routes/task/task-types';
import { clearStoredTaskSnapshot, taskSnapshotGeneration } from '../../../src/routes/task/task-snapshot-storage';

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

	test('shows a snapshot only to the same project, company and account', () => {
		rememberTask(state, summary, 'plane:company-a:member-a');
		expect(lastSeenTask('plane:company-a:member-a').state).toBe(state);
		expect(lastSeenTask('plane:company-a:member-b').state).toBeNull();
		expect(lastSeenTask('plane:company-b:member-a').summary).toBeNull();
	});

	test('signing out leaves nothing for the next account to open on', () => {
		rememberTask(state, summary);
		forgetLastSeenTask();
		expect(lastSeenTask().state).toBe(null);
	});
	test('a write invalidates memory and a pre-write response cannot remember it again', () => {
		const generation = taskSnapshotGeneration();
		rememberTask(state, summary, 'scope', generation);
		clearStoredTaskSnapshot();
		expect(lastSeenTask('scope').state).toBeNull();
		rememberTask(state, summary, 'scope', generation);
		expect(lastSeenTask('scope').state).toBeNull();
	});
	test('permission denial clears only the denied memory scope', () => {
		rememberTask(state, summary, 'scope');
		forgetLastSeenTask('scope');
		expect(lastSeenTask('scope').state).toBeNull();
	});
});
