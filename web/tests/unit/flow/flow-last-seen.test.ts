import { describe, expect, test } from 'bun:test';
import { forgetLastSeenFlow, lastSeenFlow, rememberFlow } from '../../../src/routes/flow/flow-last-seen';
import type { FlowState, FlowSummary } from '../../../src/routes/flow/flow-types';

const state = { tasks: [], members: [] } as unknown as FlowState;
const summary = { tasks: [], weeklyTasks: [] } as unknown as FlowSummary;

describe('what the flow screen shows before its refresh arrives', () => {
	test('nothing has been seen until a load succeeds', () => {
		forgetLastSeenFlow();
		expect(lastSeenFlow()).toEqual({ state: null, summary: null });
	});

	test('a finished load is what the next visit paints straight away', () => {
		rememberFlow(state, summary);
		expect(lastSeenFlow().state).toBe(state);
		expect(lastSeenFlow().summary).toBe(summary);
	});

	test('signing out leaves nothing for the next account to open on', () => {
		rememberFlow(state, summary);
		forgetLastSeenFlow();
		expect(lastSeenFlow().state).toBe(null);
	});
});
