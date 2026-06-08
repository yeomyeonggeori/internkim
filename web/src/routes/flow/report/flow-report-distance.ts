import { isFlowStatusCompleted, isFlowStatusInProgress } from '../flow-status';
import type { FlowReportDefinitions, FlowReportTask } from './flow-report-types';

export function distanceForTaskProgress(task: FlowReportTask, definitions: FlowReportDefinitions): number {
	const distance = distanceForTaskSize(task.size, definitions);
	if (isFlowStatusCompleted(task.status)) return distance;
	if (isFlowStatusInProgress(task.status)) return Math.floor(distance / 2);
	return 0;
}

export function taskTeamDistance(task: FlowReportTask, definitions: FlowReportDefinitions): number {
	return distanceForTaskProgress(task, definitions) * participantCount(task);
}

export function weeklyTaskDayIndex(task: FlowReportTask, weekStartISO: string | undefined): number {
	if (!weekStartISO) return -1;
	const taskDate = distanceDateForTask(task);
	if (!taskDate) return -1;
	return dateOffset(weekStartISO, taskDate);
}

function participantCount(task: FlowReportTask): number {
	return Math.max(1, task.participantNames.length);
}

function distanceDateForTask(task: FlowReportTask): string {
	if (isFlowStatusCompleted(task.status) && task.endDate?.trim()) return task.endDate.trim();
	return task.startDate?.trim() ?? '';
}

function dateOffset(startISO: string, value: string): number {
	const startDate = Date.parse(`${startISO}T00:00:00Z`);
	const targetDate = Date.parse(`${value}T00:00:00Z`);
	if (!Number.isFinite(startDate) || !Number.isFinite(targetDate)) return -1;
	return Math.floor((targetDate - startDate) / 86_400_000);
}

function distanceForTaskSize(sizeName: string, definitions: FlowReportDefinitions): number {
	const normalizedName = sizeName.trim().toUpperCase();
	const found = definitions.sizes.find((size) => size.name.trim().toUpperCase() === normalizedName);
	return found?.distanceKm ?? 0;
}
