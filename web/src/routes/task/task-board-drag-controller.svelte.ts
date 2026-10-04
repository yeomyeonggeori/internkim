import { feelHaptic } from '$lib/native-shell/haptics';
import type { TaskBoardMoveRequest } from './task-board-drag';
import { isTaskBoardStatus } from './task-board-model';
import type { Task } from './task-types';

type TaskBoardDragControllerInput = {
	isTaskPending: (taskID: string) => boolean;
	canUpdateTask: (task: Task) => boolean;
	moveTask: (request: TaskBoardMoveRequest) => void | Promise<void>;
};

const boardDragDataType = 'application/x-internkim-task-id';

export class TaskBoardDragController {
	private draggedTaskID = $state('');
	private readonly input: TaskBoardDragControllerInput;

	constructor(input: TaskBoardDragControllerInput) {
		this.input = input;
	}

	handleTaskDragStart = (event: DragEvent, task: Task): void => {
		if (this.input.isTaskPending(task.id) || !this.input.canUpdateTask(task)) {
			event.preventDefault();
			return;
		}
		this.draggedTaskID = task.id;
		feelHaptic('touch');
		event.dataTransfer?.setData(boardDragDataType, task.id);
		event.dataTransfer?.setData('text/plain', task.id);
		if (event.dataTransfer) event.dataTransfer.effectAllowed = 'move';
	};

	handleTaskDragEnd = (): void => {
		this.draggedTaskID = '';
	};

	handleColumnDragOver = (event: DragEvent, status: string, columnTasks: Task[]): void => {
		const taskID = this.currentDragTaskID(event);
		if (!taskID || this.input.isTaskPending(taskID) || !isTaskBoardStatus(status)) return;
		if (this.isSameColumnDrag(taskID, columnTasks)) return;
		event.preventDefault();
		if (event.dataTransfer) event.dataTransfer.dropEffect = 'move';
	};

	handleColumnDrop = (event: DragEvent, status: string, columnTasks: Task[]): void => {
		event.preventDefault();
		const taskID = this.currentDragTaskID(event);
		if (!taskID || this.input.isTaskPending(taskID) || !isTaskBoardStatus(status)) return;
		if (this.isSameColumnDrag(taskID, columnTasks)) {
			this.handleTaskDragEnd();
			return;
		}
		feelHaptic('touch');
		void this.input.moveTask({ taskID, targetStatus: status });
		this.handleTaskDragEnd();
	};

	private currentDragTaskID(event: DragEvent): string {
		return this.draggedTaskID || event.dataTransfer?.getData(boardDragDataType) || '';
	}

	private isSameColumnDrag(taskID: string, columnTasks: Task[]): boolean {
		return columnTasks.some((task) => task.id === taskID);
	}
}
