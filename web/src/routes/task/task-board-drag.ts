import { isTaskBoardStatus } from './task-board-model';
import type { Task } from './task-types';

export type TaskBoardMoveRequest = {
	taskID: string;
	targetStatus: string;
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
	const movedTask = tasks.find((task) => task.id === request.taskID);
	if (!movedTask) return null;
	if (movedTask.status === request.targetStatus) return null;

	const movedTargetTask = { ...movedTask, status: request.targetStatus };
	return {
		tasks: tasks.map((task) => (task.id === movedTargetTask.id ? movedTargetTask : task)),
		updates: [movedTargetTask]
	};
}
