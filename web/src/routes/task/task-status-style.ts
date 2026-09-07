import CalendarIcon from '@lucide/svelte/icons/calendar';
import CheckIcon from '@lucide/svelte/icons/check';
import LoaderIcon from '@lucide/svelte/icons/loader';
import PauseIcon from '@lucide/svelte/icons/pause';
import SendHorizontalIcon from '@lucide/svelte/icons/send-horizontal';
import XIcon from '@lucide/svelte/icons/x';
import {
	isTaskStatusCompleted,
	isTaskStatusInProgress,
	isTaskStatusPaused,
	isTaskStatusRejected,
	isTaskStatusRequested,
	isTaskStatusStopped
} from './task-status';

export type TaskStatusIcon = typeof CheckIcon;
export type TaskStatusVariant = 'default' | 'secondary' | 'destructive' | 'outline';

const taskStatusOrder: Array<(status: string) => boolean> = [
	(status) => isTaskStatusRejected(status) || isTaskStatusStopped(status),
	isTaskStatusCompleted,
	isTaskStatusPaused,
	isTaskStatusInProgress
];

export function taskStatusIcon(status: string): TaskStatusIcon {
	if (isTaskStatusCompleted(status)) return CheckIcon;
	if (isTaskStatusRejected(status) || isTaskStatusStopped(status)) return XIcon;
	if (isTaskStatusPaused(status)) return PauseIcon;
	if (isTaskStatusInProgress(status)) return LoaderIcon;
	if (isTaskStatusRequested(status)) return SendHorizontalIcon;
	return CalendarIcon;
}

export function taskStatusVariant(status: string): TaskStatusVariant {
	if (isTaskStatusInProgress(status)) return 'default';
	if (isTaskStatusRejected(status) || isTaskStatusStopped(status)) return 'destructive';
	if (isTaskStatusCompleted(status)) return 'secondary';
	return 'outline';
}

export function taskStatusRank(status: string): number {
	const matched = taskStatusOrder.findIndex((matches) => matches(status));
	return matched === -1 ? taskStatusOrder.length : matched;
}
