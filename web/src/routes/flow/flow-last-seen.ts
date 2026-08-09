import type { FlowState, FlowSummary } from './flow-types';

let lastState: FlowState | null = null;
let lastSummary: FlowSummary | null = null;

export function lastSeenFlow(): { state: FlowState | null; summary: FlowSummary | null } {
	return { state: lastState, summary: lastSummary };
}

export function rememberFlow(state: FlowState, summary: FlowSummary): void {
	lastState = state;
	lastSummary = summary;
}

export function forgetLastSeenFlow(): void {
	lastState = null;
	lastSummary = null;
}
