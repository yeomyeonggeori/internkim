import { buildFlowTaskBoard, isFlowTaskBoardStatus } from './flow-task-board-model';
import type { FlowTask } from './flow-types';

export const flowTaskBoardRankStep = 1024;

export type FlowTaskBoardMoveRequest = {
	taskID: string;
	targetStatus: string;
	beforeTaskID: string | null;
};

export type FlowTaskBoardMoveResult = {
	tasks: FlowTask[];
	updates: FlowTask[];
};

export function createFlowTaskBoardMove(
	tasks: FlowTask[],
	request: FlowTaskBoardMoveRequest
): FlowTaskBoardMoveResult | null {
	if (!isFlowTaskBoardStatus(request.targetStatus)) return null;
	if (request.beforeTaskID === request.taskID) return null;
	const movedTask = tasks.find((task) => task.id === request.taskID);
	if (!movedTask) return null;

	const targetTasks = boardTasksForStatus(tasks, request.targetStatus)
		.filter((task) => task.id !== request.taskID);
	const insertIndex = targetInsertIndex(targetTasks, request.beforeTaskID);
	const movedTargetTask = { ...movedTask, status: request.targetStatus };
	const reorderedTasks = [
		...targetTasks.slice(0, insertIndex),
		movedTargetTask,
		...targetTasks.slice(insertIndex)
	];

	if (
		movedTask.status === request.targetStatus
		&& sameTaskOrder(boardTasksForStatus(tasks, request.targetStatus), reorderedTasks)
	) {
		return null;
	}

	const movedStatusRank = statusRankBetween(targetTasks[insertIndex - 1], targetTasks[insertIndex]);
	if (movedStatusRank !== null) {
		const rankedMovedTask = { ...movedTargetTask, statusRank: movedStatusRank };
		if (!hasFlowTaskBoardMoveChange(rankedMovedTask, tasks)) return null;
		return {
			tasks: tasks.map((task) => (task.id === rankedMovedTask.id ? rankedMovedTask : task)),
			updates: [rankedMovedTask]
		};
	}

	const rankedTasks = reorderedTasks.map((task, index) => ({
		...task,
		statusRank: (index + 1) * flowTaskBoardRankStep
	}));
	const rankedTaskByID = new Map(rankedTasks.map((task) => [task.id, task]));
	const nextTasks = tasks.map((task) => rankedTaskByID.get(task.id) ?? task);
	const updates = prioritizeMovedTask(
		rankedTasks.filter((task) => hasFlowTaskBoardMoveChange(task, tasks)),
		request.taskID
	);
	if (updates.length === 0) return null;

	return {
		tasks: nextTasks,
		updates
	};
}

function boardTasksForStatus(tasks: FlowTask[], status: string): FlowTask[] {
	return buildFlowTaskBoard(tasks).find((column) => column.status === status)?.tasks ?? [];
}

function targetInsertIndex(tasks: FlowTask[], beforeTaskID: string | null): number {
	if (!beforeTaskID) return tasks.length;
	const index = tasks.findIndex((task) => task.id === beforeTaskID);
	return index < 0 ? tasks.length : index;
}

function sameTaskOrder(left: FlowTask[], right: FlowTask[]): boolean {
	if (left.length !== right.length) return false;
	return left.every((task, index) => task.id === right[index]?.id);
}

function statusRankBetween(previousTask: FlowTask | undefined, nextTask: FlowTask | undefined): number | null {
	if (!previousTask && !nextTask) return flowTaskBoardRankStep;
	if (!previousTask) return rankBefore(nextTask);
	if (!nextTask) return previousTask.statusRank + flowTaskBoardRankStep;
	return rankBetween(previousTask, nextTask);
}

function rankBefore(task: FlowTask | undefined): number | null {
	if (!task || task.statusRank <= 1) return null;
	return Math.floor(task.statusRank / 2);
}

function rankBetween(previousTask: FlowTask, nextTask: FlowTask): number | null {
	const distance = nextTask.statusRank - previousTask.statusRank;
	if (distance <= 1) return null;
	return previousTask.statusRank + Math.floor(distance / 2);
}

function prioritizeMovedTask(tasks: FlowTask[], movedTaskID: string): FlowTask[] {
	return [...tasks].sort((left, right) => {
		if (left.id === movedTaskID) return -1;
		if (right.id === movedTaskID) return 1;
		return 0;
	});
}

function hasFlowTaskBoardMoveChange(task: FlowTask, previousTasks: FlowTask[]): boolean {
	const previousTask = previousTasks.find((value) => value.id === task.id);
	if (!previousTask) return false;
	return previousTask.status !== task.status || previousTask.statusRank !== task.statusRank;
}
