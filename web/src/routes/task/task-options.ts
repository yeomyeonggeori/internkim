import { buildBusinessSelectOptions } from './task-workspace-model';
import { taskStatus } from './task-status';
import { isCentralTaskSource } from './task-source';
import { taskText } from './text';
import type { TaskDefinitions, TaskSummary, Task } from './task-types';

type TaskPageText = typeof taskText.ko;

const emptyDefinitions: TaskDefinitions = {
	categories: [],
	types: [],
	sizes: []
};

export function definitionsFromSummary(summary: TaskSummary | null): TaskDefinitions {
	return summary?.definitions ?? emptyDefinitions;
}

export function statusOptionsFromSummary(summary: TaskSummary | null, task?: Task | null): string[] {
	if (!summary) return [];
	if (!isCentralTaskSource(summary.source)) return summary.statusOptions;
	if (hasTaskRequestProvenance(task)) {
		return [
			taskStatus.requested,
			taskStatus.planned,
			taskStatus.inProgress,
			taskStatus.completed,
			taskStatus.paused,
			taskStatus.rejected,
			taskStatus.stopped
		];
	}
	return [taskStatus.planned, taskStatus.inProgress, taskStatus.completed, taskStatus.paused, taskStatus.stopped];
}

export function hasTaskRequestProvenance(task?: Task | null): boolean {
	return Boolean(task?.requesterID);
}

export function taskStatusLabel(text: TaskPageText, status: string): string {
	const labels = text.status as Record<string, string>;
	return labels[status] ?? status;
}

export function categorySelectOptions(definitions: TaskDefinitions, fallbackBusiness: string) {
	return buildBusinessSelectOptions(definitions, fallbackBusiness);
}

export function typeSelectOptions(definitions: TaskDefinitions) {
	return definitions.types.map((type) => ({ value: type, label: type }));
}

export function sizeSelectOptions(definitions: TaskDefinitions) {
	return definitions.sizes.map((size) => ({ value: size.name, label: `${size.name} · ${size.distanceKm}km · ${size.maxHours}h` }));
}

export function statusSelectOptions(statuses: string[], statusLabel: (status: string) => string) {
	return statuses.map((status) => ({ value: status, label: statusLabel(status) }));
}
