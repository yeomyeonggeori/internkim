import type { TaskState, TaskSummary } from './task-types';
import { clearStoredTaskSnapshot, storedTaskSnapshot, storeTaskSnapshot, taskSnapshotGeneration } from './task-snapshot-storage';

let lastState: TaskState | null = null;
let lastSummary: TaskSummary | null = null;
let lastScope = '';
let savedAt = 0;
let rememberedGeneration = 0;

export function lastSeenTask(scope = ''): { state: TaskState | null; summary: TaskSummary | null; savedAt?: number } {
	if (scope === lastScope && lastState && rememberedGeneration === taskSnapshotGeneration() && Date.now() - savedAt <= 86400000) return { state: lastState, summary: lastSummary, savedAt };
	const stored = storedTaskSnapshot(scope);
	return stored ? { state: stored.state, summary: null, savedAt: stored.savedAt } : { state: null, summary: null };
}

export function rememberTask(state: TaskState, summary: TaskSummary, scope = '', readGeneration = taskSnapshotGeneration()): void {
	if (readGeneration !== taskSnapshotGeneration()) return;
	rememberedGeneration = readGeneration;
	lastScope = scope;
	lastState = state;
	lastSummary = summary;
	savedAt = Date.now();
	storeTaskSnapshot(scope, state, savedAt);
}

export function forgetLastSeenTask(scope?: string): void {
	if (!scope || scope === lastScope) {
		lastScope = '';
		lastState = null;
		lastSummary = null;
		savedAt = 0;
	}
	clearStoredTaskSnapshot(scope);
}
