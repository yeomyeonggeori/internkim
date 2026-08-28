import { updateTaskParent, updateTaskParents } from './task-api';
import type { LoadTask } from './task-load-tracker';
import type { Task } from './task-types';

type TaskRelationshipControllerInput = {
	loadTask: LoadTask;
	weekCode: () => string;
	fallbackMessage: () => string;
	setErrorMessage: (message: string) => void;
};

export class TaskRelationshipController {
	pendingTaskIDs = $state<string[]>([]);

	private loadTask: LoadTask = async () => false;
	private weekCode: () => string = () => '';
	private fallbackMessage: () => string = () => '';
	private setErrorMessage: (message: string) => void = () => {};

	sync(input: TaskRelationshipControllerInput): void {
		this.loadTask = input.loadTask;
		this.weekCode = input.weekCode;
		this.fallbackMessage = input.fallbackMessage;
		this.setErrorMessage = input.setErrorMessage;
	}

	setParent = async (taskID: string, parentTaskID?: string): Promise<boolean> => {
		if (!taskID || this.pendingTaskIDs.includes(taskID)) return false;
		this.pendingTaskIDs = [...this.pendingTaskIDs, taskID];
		this.setErrorMessage('');
		try {
			await updateTaskParent(taskID, parentTaskID, this.fallbackMessage());
			await this.loadTask(this.weekCode(), { reloadState: true });
			return true;
		} catch (error) {
			this.setErrorMessage(error instanceof Error ? error.message : this.fallbackMessage());
			return false;
		} finally {
			this.pendingTaskIDs = this.pendingTaskIDs.filter((pendingTaskID) => pendingTaskID !== taskID);
		}
	};

	setParents = async (taskIDs: string[], parentTaskID: string): Promise<boolean> => {
		const uniqueTaskIDs = [...new Set(taskIDs)].filter((taskID) => taskID && !this.pendingTaskIDs.includes(taskID));
		if (!parentTaskID || uniqueTaskIDs.length !== taskIDs.length) return false;
		this.pendingTaskIDs = [...this.pendingTaskIDs, ...uniqueTaskIDs];
		this.setErrorMessage('');
		try {
			await updateTaskParents(uniqueTaskIDs, parentTaskID, this.fallbackMessage());
			await this.loadTask(this.weekCode(), { reloadState: true });
			return true;
		} catch (error) {
			this.setErrorMessage(error instanceof Error ? error.message : this.fallbackMessage());
			return false;
		} finally {
			this.pendingTaskIDs = this.pendingTaskIDs.filter((pendingTaskID) => !uniqueTaskIDs.includes(pendingTaskID));
		}
	};

	isPending(task: Task): boolean {
		return this.pendingTaskIDs.includes(task.id);
	}
}
