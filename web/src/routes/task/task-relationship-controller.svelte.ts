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

	setParent = (taskID: string, parentTaskID?: string): Promise<boolean> => {
		if (!taskID || this.pendingTaskIDs.includes(taskID)) return Promise.resolve(false);
		return this.recordThenReload([taskID], () => updateTaskParent(taskID, parentTaskID));
	};

	setParents = (taskIDs: string[], parentTaskID: string): Promise<boolean> => {
		const uniqueTaskIDs = [...new Set(taskIDs)].filter((taskID) => taskID && !this.pendingTaskIDs.includes(taskID));
		if (!parentTaskID || uniqueTaskIDs.length !== taskIDs.length) return Promise.resolve(false);
		return this.recordThenReload(uniqueTaskIDs, () => updateTaskParents(uniqueTaskIDs, parentTaskID));
	};

	private async recordThenReload(taskIDs: string[], record: () => Promise<void>): Promise<boolean> {
		this.pendingTaskIDs = [...this.pendingTaskIDs, ...taskIDs];
		this.setErrorMessage('');
		try {
			await record();
		} catch (error) {
			this.setErrorMessage(error instanceof Error ? error.message : this.fallbackMessage());
			this.release(taskIDs);
			return false;
		}
		void this.loadTask(this.weekCode(), { reloadState: true }).finally(() => this.release(taskIDs));
		return true;
	}

	private release(taskIDs: string[]): void {
		this.pendingTaskIDs = this.pendingTaskIDs.filter((pendingTaskID) => !taskIDs.includes(pendingTaskID));
	}

	isPending(task: Task): boolean {
		return this.pendingTaskIDs.includes(task.id);
	}
}
