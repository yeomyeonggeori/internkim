import { expect, test } from 'bun:test';

import {
	createCalendarConflictActions,
	type CalendarConflictActionContext
} from '../../../src/routes/calendar/embed/calendar-conflict-actions';
import type { CalendarConflict } from '../../../src/routes/calendar/embed/calendar-conflicts';

const firstConflict: CalendarConflict = {
	id: 1,
	eventID: 'event-1',
	eventUID: 'event-1@internkim',
	field: 'title',
	localValue: 'Local',
	remoteValue: 'Remote',
	detectedAt: '2026-06-02T00:00:00Z'
};

const secondConflict: CalendarConflict = {
	...firstConflict,
	id: 2,
	eventID: 'event-2',
	eventUID: 'event-2@internkim'
};

function createConflictActionTester(initialConflicts: CalendarConflict[] = [], isBrowser = true) {
	let conflicts = initialConflicts;
	let errorMessage = '';
	const calls: string[] = [];
	const context: CalendarConflictActionContext = {
		isBrowser: () => isBrowser,
		errorFallback: () => 'Fallback error',
		getCalendarConflicts: () => conflicts,
		setCalendarConflicts: (nextConflicts) => {
			calls.push(`set:${nextConflicts.length}`);
			conflicts = nextConflicts;
		},
		setErrorMessage: (message) => {
			calls.push(`error:${message}`);
			errorMessage = message;
		},
		syncRemoteCalendarAndRefresh: async () => {
			calls.push('sync');
		}
	};
	return {
		calls,
		context,
		currentConflicts: () => conflicts,
		errorMessage: () => errorMessage
	};
}

test('loads conflicts through the action boundary', async () => {
	const tester = createConflictActionTester();
	const actions = createCalendarConflictActions(tester.context, {
		fetchCalendarConflicts: async () => {
			tester.calls.push('fetch');
			return [firstConflict];
		}
	});

	await actions.loadCalendarConflicts();

	expect(tester.calls).toEqual(['fetch', 'set:1']);
	expect(tester.currentConflicts()).toEqual([firstConflict]);
});

test('skips loading conflicts outside the browser', async () => {
	const tester = createConflictActionTester([], false);
	const actions = createCalendarConflictActions(tester.context, {
		fetchCalendarConflicts: async () => {
			tester.calls.push('fetch');
			return [firstConflict];
		}
	});

	await actions.loadCalendarConflicts();

	expect(tester.calls).toEqual([]);
	expect(tester.currentConflicts()).toEqual([]);
});

test('dismisses one conflict and updates local state', async () => {
	const tester = createConflictActionTester([firstConflict, secondConflict]);
	const actions = createCalendarConflictActions(tester.context, {
		dismissCalendarConflictOnServer: async (conflictID) => {
			tester.calls.push(`dismiss:${conflictID}`);
		}
	});

	await actions.dismissCalendarConflict(1);

	expect(tester.calls).toEqual(['dismiss:1', 'set:1']);
	expect(tester.currentConflicts()).toEqual([secondConflict]);
});

test('dismisses all conflicts then delegates remote sync refresh', async () => {
	const tester = createConflictActionTester([firstConflict]);
	const actions = createCalendarConflictActions(tester.context, {
		dismissCalendarConflictsOnServer: async (conflictIDs) => {
			tester.calls.push(`dismiss-all:${conflictIDs.join(',')}`);
		}
	});

	await actions.dismissAllConflictsAndRefresh();

	expect(tester.calls).toEqual(['dismiss-all:1', 'set:0', 'sync']);
	expect(tester.currentConflicts()).toEqual([]);
});

test('reloads conflicts after dismiss-all fails', async () => {
	const tester = createConflictActionTester([firstConflict]);
	const actions = createCalendarConflictActions(tester.context, {
		dismissCalendarConflictsOnServer: async () => {
			tester.calls.push('dismiss-all');
			throw new Error('Dismiss failed');
		},
		fetchCalendarConflicts: async () => {
			tester.calls.push('fetch');
			return [firstConflict];
		}
	});

	await actions.dismissAllConflictsAndRefresh();

	expect(tester.calls).toEqual(['dismiss-all', 'error:Dismiss failed', 'fetch', 'set:1']);
	expect(tester.errorMessage()).toBe('Dismiss failed');
	expect(tester.currentConflicts()).toEqual([firstConflict]);
});
