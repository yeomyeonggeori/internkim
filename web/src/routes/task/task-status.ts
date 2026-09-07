export const taskStatus = {
	requested: 'requested',
	planned: 'planned',
	inProgress: 'in_progress',
	completed: 'completed',
	paused: 'paused',
	rejected: 'rejected',
	stopped: 'stopped'
} as const;

export type TaskStatus = (typeof taskStatus)[keyof typeof taskStatus];

export function cleanTaskStatus(status: string): string {
	return status.trim().toLowerCase();
}

export function isTaskStatusCompleted(status: string): boolean {
	return cleanTaskStatus(status) === taskStatus.completed;
}

export function isTaskStatusInProgress(status: string): boolean {
	return cleanTaskStatus(status) === taskStatus.inProgress;
}

export function isTaskStatusPlanned(status: string): boolean {
	return cleanTaskStatus(status) === taskStatus.planned;
}

export function isTaskStatusRequested(status: string): boolean {
	return cleanTaskStatus(status) === taskStatus.requested;
}

export function isTaskStatusPaused(status: string): boolean {
	return cleanTaskStatus(status) === taskStatus.paused;
}

export function isTaskStatusRejected(status: string): boolean {
	return cleanTaskStatus(status) === taskStatus.rejected;
}

export function isTaskStatusStopped(status: string): boolean {
	return cleanTaskStatus(status) === taskStatus.stopped;
}

export function isTaskStatusFinished(status: string): boolean {
	return isTaskStatusCompleted(status) || isTaskStatusRejected(status) || isTaskStatusStopped(status);
}

const statusWordsWithoutLabel = new Set<string>();

export function taskStatusLabelFrom(labels: Record<string, string>, status: string): string {
	const label = labels[cleanTaskStatus(status)];
	if (label) return label;
	if (!statusWordsWithoutLabel.has(status)) {
		statusWordsWithoutLabel.add(status);
		console.warn(`no label for the task status "${status}"`);
	}
	return status;
}
