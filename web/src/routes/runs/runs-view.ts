import type { Component } from 'svelte';
import BanIcon from '@lucide/svelte/icons/ban';
import CheckIcon from '@lucide/svelte/icons/check';
import ClockIcon from '@lucide/svelte/icons/clock';
import HourglassIcon from '@lucide/svelte/icons/hourglass';
import LoaderIcon from '@lucide/svelte/icons/loader';
import MinusIcon from '@lucide/svelte/icons/minus';
import XIcon from '@lucide/svelte/icons/x';
import type { TasksText } from './text';
import type { EventLane } from './runs-api';

export function shortTaskRunID(taskRunID: string): string {
	return taskRunID.length > 6 ? taskRunID.slice(0, 6) : taskRunID;
}

export function taskStatusToneClass(status: string): string {
	switch (status) {
		case 'failed':
			return 'text-destructive font-medium';
		case 'waiting_user_input':
		case 'waiting_approval':
			return 'text-warning-subtle-foreground';
		default:
			return 'text-muted-foreground';
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

export function formatTaskTimestamp(value?: string, now = new Date(), locale?: string): string {
	if (!value) return '';
	const parsed = new Date(value);
	if (Number.isNaN(parsed.getTime())) return value;
	const time: Intl.DateTimeFormatOptions = { hour: 'numeric', minute: '2-digit' };
	if (parsed.toDateString() === now.toDateString()) return parsed.toLocaleTimeString(locale, time);
	const day: Intl.DateTimeFormatOptions = { month: 'short', day: 'numeric', ...time };
	if (parsed.getFullYear() === now.getFullYear()) return parsed.toLocaleString(locale, day);
	return parsed.toLocaleDateString(locale, { year: 'numeric', month: 'short', day: 'numeric' });
}

export function formatDuration(durationMS?: number): string {
	if (durationMS === undefined || !Number.isFinite(durationMS) || durationMS < 100) return '';
	return formatLatency(durationMS);
}

export function formatEventClock(value?: string): string {
	const parsed = new Date(value ?? '');
	if (Number.isNaN(parsed.getTime())) return '';
	return parsed.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false, fractionalSecondDigits: 3 });
}

export function formatLatency(latencyMS: number): string {
	if (latencyMS >= 1000) return `${(latencyMS / 1000).toFixed(1)}s`;
	return `${latencyMS}ms`;
}
