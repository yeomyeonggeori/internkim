import { isFlowStatusCompleted } from '../flow-status';
import type { FlowReportDefinitions, FlowReportTask } from './flow-report-types';

export function completedDistanceForTask(task: FlowReportTask, definitions: FlowReportDefinitions): number {
	if (!completedTaskEndDate(task)) return 0;
	return distanceForTaskSize(task.size, definitions);
}

export function taskTeamDistance(task: FlowReportTask, definitions: FlowReportDefinitions): number {
	return completedDistanceForTask(task, definitions);
}

export function weeklyTaskDayIndex(task: FlowReportTask, weekStartISO: string | undefined): number {
	if (!weekStartISO) return -1;
	const taskDate = completedTaskEndDate(task);
	if (!taskDate) return -1;
	return dateOffset(weekStartISO, taskDate);
}

function completedTaskEndDate(task: FlowReportTask): string {
	if (!isFlowStatusCompleted(task.status)) return '';
	const value = task.endDate?.trim() ?? '';
	if (!Number.isFinite(Date.parse(`${value}T00:00:00Z`))) return '';
	return value;
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
