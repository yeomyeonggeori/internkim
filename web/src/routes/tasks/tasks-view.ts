import type { Component } from 'svelte';
import BanIcon from '@lucide/svelte/icons/ban';
import CircleCheckIcon from '@lucide/svelte/icons/circle-check';
import CircleHelpIcon from '@lucide/svelte/icons/circle-help';
import CirclePauseIcon from '@lucide/svelte/icons/circle-pause';
import CircleXIcon from '@lucide/svelte/icons/circle-x';
import ClockIcon from '@lucide/svelte/icons/clock';
import LoaderCircleIcon from '@lucide/svelte/icons/loader-circle';
import CircleIcon from '@lucide/svelte/icons/circle';
import type { BadgeVariant } from '$lib/components/ui/badge';
import type { TasksText } from './text';
import type { EventLane } from './tasks-api';

export function shortTaskRunID(taskRunID: string): string {
	return taskRunID.length > 6 ? taskRunID.slice(0, 6) : taskRunID;
}

export function taskStatusBadgeVariant(status: string): BadgeVariant {
	switch (status) {
		case 'completed':
			return 'default';
		case 'failed':
			return 'destructive';
		case 'running':
		case 'planned':
			return 'secondary';
		default:
			return 'outline';
	}
}

export function taskStatusIcon(status: string): Component {
	switch (status) {
		case 'completed':
			return CircleCheckIcon;
		case 'failed':
		case 'cancelled':
			return CircleXIcon;
		case 'running':
			return LoaderCircleIcon;
		case 'planned':
			return ClockIcon;
		case 'waiting_user_input':
		case 'waiting_approval':
			return CircleHelpIcon;
		case 'blocked':
			return BanIcon;
		case 'interrupted':
			return CirclePauseIcon;
		default:
			return CircleIcon;
	}
}

export function taskStatusLabel(status: string, text: TasksText): string {
	switch (status) {
		case 'completed':
			return text.statusCompleted;
		case 'failed':
			return text.statusFailed;
		case 'running':
			return text.statusRunning;
		case 'planned':
			return text.statusPlanned;
		case 'waiting_user_input':
			return text.statusWaitingUserInput;
		case 'waiting_approval':
			return text.statusWaitingApproval;
		case 'blocked':
			return text.statusBlocked;
		case 'interrupted':
			return text.statusInterrupted;
		case 'cancelled':
			return text.statusCancelled;
		default:
			return status;
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
