import { centralTaskStatusOptions } from '$lib/task/central-task';
import { buildBusinessSelectOptions, buildTypeSelectOptions } from './task-workspace-model';
import { taskStatus } from './task-status';
import { taskText } from './text';
import type { TaskDefinitions, TaskSummary, Task } from './task-types';
import type { PageText } from '$lib/i18n/page-text.svelte';

type TaskPageText = PageText<typeof taskText>;

const emptyDefinitions: TaskDefinitions = {
	categories: [],
	types: [],
	sizes: []
};

export function definitionsFromSummary(summary: TaskSummary | null): TaskDefinitions {
	return summary?.definitions ?? emptyDefinitions;
}

const requestLifecycleStatuses: string[] = [taskStatus.requested, taskStatus.rejected];

export function statusOptionsFromSummary(summary: TaskSummary | null, task?: Task | null): string[] {
	if (!summary) return [];
	if (hasTaskRequestProvenance(task)) return [...centralTaskStatusOptions];
	return centralTaskStatusOptions.filter((status) => !requestLifecycleStatuses.includes(status));
}

export function hasTaskRequestProvenance(task?: Task | null): boolean {
	return Boolean(task?.requesterID);
}

export function taskStatusLabel(text: TaskPageText, status: string): string {
	const labels = text.status as Record<string, string>;
	return labels[status] ?? status;
}

export function categorySelectOptions(definitions: TaskDefinitions, etcLabel: string) {
	return buildBusinessSelectOptions(definitions, etcLabel);
}

export function typeSelectOptions(definitions: TaskDefinitions, etcLabel: string) {
	return buildTypeSelectOptions(definitions, etcLabel);
}

export function sizeSelectOptions(definitions: TaskDefinitions) {
	return definitions.sizes.map((size) => ({ value: size.name, label: `${size.name} · ${size.distanceKm}km · ${size.maxHours}h` }));
}

export function statusSelectOptions(statuses: string[], statusLabel: (status: string) => string) {
	return statuses.map((status) => ({ value: status, label: statusLabel(status) }));
}
