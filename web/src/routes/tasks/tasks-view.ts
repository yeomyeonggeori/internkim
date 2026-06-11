import type { EventLane } from './tasks-api';

export function shortTaskRunID(taskRunID: string): string {
	return taskRunID.length > 6 ? taskRunID.slice(0, 6) : taskRunID;
}

export function taskStatusBadgeClass(status: string): string {
	switch (status) {
		case 'completed':
			return 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900/50 dark:text-emerald-200';
		case 'failed':
			return 'bg-red-100 text-red-800 dark:bg-red-900/50 dark:text-red-200';
		case 'blocked':
		case 'cancelled':
			return 'bg-amber-100 text-amber-800 dark:bg-amber-900/50 dark:text-amber-200';
		case 'running':
		case 'planned':
			return 'bg-blue-100 text-blue-800 dark:bg-blue-900/50 dark:text-blue-200';
		default:
			return 'bg-violet-100 text-violet-800 dark:bg-violet-900/50 dark:text-violet-200';
	}
}

export function eventLaneClass(lane: EventLane): string {
	switch (lane) {
		case 'llm':
			return 'border-l-blue-500';
		case 'tool':
			return 'border-l-emerald-500';
		case 'failure':
			return 'border-l-red-500';
		default:
			return 'border-l-zinc-400';
	}
}

export function formatTaskTimestamp(value?: string): string {
	if (!value) return '';
	const parsed = new Date(value);
	if (Number.isNaN(parsed.getTime())) return value;
	return parsed.toLocaleString();
}

export function formatLatency(latencyMS: number): string {
	if (latencyMS >= 1000) return `${(latencyMS / 1000).toFixed(1)}s`;
	return `${latencyMS}ms`;
}
