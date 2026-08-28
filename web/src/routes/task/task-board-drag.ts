import { buildTaskBoard, isTaskBoardStatus } from './task-board-model';
import type { Task } from './task-types';

export const taskBoardRankStep = 1024;

export type TaskBoardMoveRequest = {
	taskID: string;
	targetStatus: string;
	beforeTaskID: string | null;
};

export type TaskBoardMoveResult = {
	tasks: Task[];
	updates: Task[];
};

export function createTaskBoardMove(
	tasks: Task[],
	request: TaskBoardMoveRequest
): TaskBoardMoveResult | null {
	if (!isTaskBoardStatus(request.targetStatus)) return null;
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
		if (!hasTaskBoardMoveChange(rankedMovedTask, tasks)) return null;
		return {
			tasks: tasks.map((task) => (task.id === rankedMovedTask.id ? rankedMovedTask : task)),
			updates: [rankedMovedTask]
		};
	}

	const rankedTasks = reorderedTasks.map((task, index) => ({
		...task,
		statusRank: (index + 1) * taskBoardRankStep
	}));
	const rankedTaskByID = new Map(rankedTasks.map((task) => [task.id, task]));
	const nextTasks = tasks.map((task) => rankedTaskByID.get(task.id) ?? task);
	const updates = prioritizeMovedTask(
		rankedTasks.filter((task) => hasTaskBoardMoveChange(task, tasks)),
		request.taskID
	);
	if (updates.length === 0) return null;

	return {
		tasks: nextTasks,
		updates
	};
}

function boardTasksForStatus(tasks: Task[], status: string): Task[] {
	return buildTaskBoard(tasks).find((column) => column.status === status)?.tasks ?? [];
}

function targetInsertIndex(tasks: Task[], beforeTaskID: string | null): number {
	if (!beforeTaskID) return tasks.length;
	const index = tasks.findIndex((task) => task.id === beforeTaskID);
	return index < 0 ? tasks.length : index;
}

function sameTaskOrder(left: Task[], right: Task[]): boolean {
	if (left.length !== right.length) return false;
	return left.every((task, index) => task.id === right[index]?.id);
}

function statusRankBetween(previousTask: Task | undefined, nextTask: Task | undefined): number | null {
	if (!previousTask && !nextTask) return taskBoardRankStep;
	if (!previousTask) return rankBefore(nextTask);
	if (!nextTask) return previousTask.statusRank + taskBoardRankStep;
	return rankBetween(previousTask, nextTask);
}

function rankBefore(task: Task | undefined): number | null {
	if (!task || task.statusRank <= 1) return null;
	return Math.floor(task.statusRank / 2);
}

function rankBetween(previousTask: Task, nextTask: Task): number | null {
	const distance = nextTask.statusRank - previousTask.statusRank;
	if (distance <= 1) return null;
	return previousTask.statusRank + Math.floor(distance / 2);
}

function prioritizeMovedTask(tasks: Task[], movedTaskID: string): Task[] {
	return [...tasks].sort((left, right) => {
		if (left.id === movedTaskID) return -1;
		if (right.id === movedTaskID) return 1;
		return 0;
	});
}

function hasTaskBoardMoveChange(task: Task, previousTasks: Task[]): boolean {
	const previousTask = previousTasks.find((value) => value.id === task.id);
	if (!previousTask) return false;
	return previousTask.status !== task.status || previousTask.statusRank !== task.statusRank;
}
