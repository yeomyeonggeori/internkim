import type { FlowTaskBoardMoveRequest } from './flow-task-board-drag';
import { isFlowTaskBoardStatus } from './flow-task-board-model';
import type { FlowTask } from './flow-types';

type FlowTaskBoardDragControllerInput = {
	pendingTaskIDs: string[];
	canUpdateTask: (task: FlowTask) => boolean;
	moveTask: (request: FlowTaskBoardMoveRequest) => void | Promise<void>;
};

const boardDragDataType = 'application/x-internkim-flow-task-id';

export class FlowTaskBoardDragController {
	dropTarget = $state<FlowTaskBoardMoveRequest | null>(null);

	private draggedTaskID = $state('');
	private pendingTaskIDs: string[] = [];
	private canUpdateTask: (task: FlowTask) => boolean;
	private moveTask: (request: FlowTaskBoardMoveRequest) => void | Promise<void>;

	constructor() {
		this.canUpdateTask = () => false;
		this.moveTask = () => {};
	}

	sync = (input: FlowTaskBoardDragControllerInput): void => {
		this.pendingTaskIDs = input.pendingTaskIDs;
		this.canUpdateTask = input.canUpdateTask;
		this.moveTask = input.moveTask;
	};

	isTaskPending = (taskID: string): boolean => this.pendingTaskIDs.includes(taskID);

	handleTaskDragStart = (event: DragEvent, task: FlowTask): void => {
		if (this.isTaskPending(task.id) || !this.canUpdateTask(task)) {
			event.preventDefault();
			return;
		}
		this.draggedTaskID = task.id;
		event.dataTransfer?.setData(boardDragDataType, task.id);
		event.dataTransfer?.setData('text/plain', task.id);
		if (event.dataTransfer) event.dataTransfer.effectAllowed = 'move';
	};

	handleTaskDragEnd = (): void => {
		this.draggedTaskID = '';
		this.dropTarget = null;
	};

	handleColumnDragOver = (event: DragEvent, status: string, columnTasks: FlowTask[]): void => {
		const taskID = this.currentDragTaskID(event);
		if (!taskID || this.isTaskPending(taskID) || !isFlowTaskBoardStatus(status)) return;
		if (this.isSameColumnDropNoOp(taskID, null, columnTasks)) {
			this.dropTarget = null;
			return;
		}
		event.preventDefault();
		if (event.dataTransfer) event.dataTransfer.dropEffect = 'move';
		this.dropTarget = { taskID, targetStatus: status, beforeTaskID: null };
	};

	handleColumnDrop = (event: DragEvent, status: string, columnTasks: FlowTask[]): void => {
		event.preventDefault();
		const taskID = this.currentDragTaskID(event);
		if (!taskID || this.isTaskPending(taskID) || !isFlowTaskBoardStatus(status)) return;
		if (this.isSameColumnDropNoOp(taskID, null, columnTasks)) {
			this.handleTaskDragEnd();
			return;
		}
		void this.moveTask({ taskID, targetStatus: status, beforeTaskID: null });
		this.handleTaskDragEnd();
	};

	handleCardDragOver = (event: DragEvent, status: string, columnTasks: FlowTask[], task: FlowTask): void => {
		const nextDropTarget = this.cardDropTarget(event, status, columnTasks, task);
		if (!nextDropTarget) {
			if (this.currentDragTaskID(event)) {
				event.stopPropagation();
				this.dropTarget = null;
			}
			return;
		}
		event.preventDefault();
		event.stopPropagation();
		if (event.dataTransfer) event.dataTransfer.dropEffect = 'move';
		this.dropTarget = nextDropTarget;
	};

	handleCardDrop = (event: DragEvent, status: string, columnTasks: FlowTask[], task: FlowTask): void => {
		const nextDropTarget = this.cardDropTarget(event, status, columnTasks, task);
		if (!nextDropTarget) {
			if (this.currentDragTaskID(event)) {
				event.preventDefault();
				event.stopPropagation();
				this.handleTaskDragEnd();
			}
			return;
		}
		event.preventDefault();
		event.stopPropagation();
		void this.moveTask(nextDropTarget);
		this.handleTaskDragEnd();
	};

	shouldShowCardInsertionLine = (status: string, taskID: string): boolean =>
		this.dropTarget?.targetStatus === status && this.dropTarget.beforeTaskID === taskID;

	shouldShowAppendInsertionLine = (status: string): boolean =>
		this.dropTarget?.targetStatus === status && this.dropTarget.beforeTaskID === null;

	private cardDropTarget(
		event: DragEvent,
		status: string,
		columnTasks: FlowTask[],
		task: FlowTask
	): FlowTaskBoardMoveRequest | null {
		const taskID = this.currentDragTaskID(event);
		if (!taskID || taskID === task.id || this.isTaskPending(taskID) || !isFlowTaskBoardStatus(status)) return null;
		const currentTarget = event.currentTarget;
		if (!(currentTarget instanceof HTMLElement)) return null;
		const bounds = currentTarget.getBoundingClientRect();
		const isAfterTask = event.clientY > bounds.top + bounds.height / 2;
		const taskIndex = columnTasks.findIndex((candidate) => candidate.id === task.id);
		if (taskIndex < 0) return null;
		const nextTask = isAfterTask ? columnTasks[taskIndex + 1] : task;
		const beforeTaskID = nextTask?.id ?? null;
		if (this.isSameColumnDropNoOp(taskID, beforeTaskID, columnTasks)) return null;
		return { taskID, targetStatus: status, beforeTaskID };
	}

	private currentDragTaskID(event: DragEvent): string {
		return this.draggedTaskID || event.dataTransfer?.getData(boardDragDataType) || '';
	}

	private isSameColumnDropNoOp(taskID: string, beforeTaskID: string | null, columnTasks: FlowTask[]): boolean {
		const draggedTaskIndex = columnTasks.findIndex((task) => task.id === taskID);
		if (draggedTaskIndex < 0) return false;
		if (beforeTaskID === null) return draggedTaskIndex === columnTasks.length - 1;
		const beforeTaskIndex = columnTasks.findIndex((task) => task.id === beforeTaskID);
		return beforeTaskIndex === draggedTaskIndex || beforeTaskIndex === draggedTaskIndex + 1;
	}
}
