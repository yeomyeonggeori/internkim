import type { Component } from 'svelte';
import BanIcon from '@lucide/svelte/icons/ban';
import CheckIcon from '@lucide/svelte/icons/check';
import ClockIcon from '@lucide/svelte/icons/clock';
import HourglassIcon from '@lucide/svelte/icons/hourglass';
import LoaderIcon from '@lucide/svelte/icons/loader';
import MinusIcon from '@lucide/svelte/icons/minus';
import XIcon from '@lucide/svelte/icons/x';
import type { BadgeVariant } from '$lib/components/ui/badge';
import type { TasksText } from './text';
import type { EventLane } from './runs-api';

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
			return CheckIcon;
		case 'failed':
		case 'cancelled':
			return XIcon;
		case 'running':
			return LoaderIcon;
		case 'planned':
			return ClockIcon;
		case 'waiting_user_input':
		case 'waiting_approval':
			return HourglassIcon;
		case 'blocked':
		case 'interrupted':
			return BanIcon;
		default:
			return MinusIcon;
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

export function formatTaskTimestamp(value?: string): string {
	if (!value) return '';
	const parsed = new Date(value);
	if (Number.isNaN(parsed.getTime())) return value;
	return parsed.toLocaleString();
}

export function formatElapsed(startedAt?: string, at?: string): string {
	const elapsedMS = Date.parse(at ?? '') - Date.parse(startedAt ?? '');
	if (!Number.isFinite(elapsedMS) || elapsedMS < 0) return '';
	const elapsedSeconds = elapsedMS / 1000;
	if (elapsedSeconds < 60) return `+${elapsedSeconds.toFixed(1)}s`;
	return `+${Math.floor(elapsedSeconds / 60)}m ${Math.round(elapsedSeconds % 60)}s`;
}

export function formatLatency(latencyMS: number): string {
	if (latencyMS >= 1000) return `${(latencyMS / 1000).toFixed(1)}s`;
	return `${latencyMS}ms`;
}
