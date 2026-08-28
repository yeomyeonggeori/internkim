import type { TaskState, TaskSummary } from './task-types';

let lastState: TaskState | null = null;
let lastSummary: TaskSummary | null = null;

export function lastSeenTask(): { state: TaskState | null; summary: TaskSummary | null } {
	return { state: lastState, summary: lastSummary };
}

export function rememberTask(state: TaskState, summary: TaskSummary): void {
	lastState = state;
	lastSummary = summary;
}

export function forgetLastSeenTask(): void {
	lastState = null;
	lastSummary = null;
}
