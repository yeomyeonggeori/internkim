import { updateFlowTaskParent, updateFlowTaskParents } from './flow-api';
import type { LoadFlow } from './flow-load-tracker';
import type { FlowTask } from './flow-types';

type FlowTaskRelationshipControllerInput = {
	loadFlow: LoadFlow;
	weekCode: () => string;
	fallbackMessage: () => string;
	setErrorMessage: (message: string) => void;
};

export class FlowTaskRelationshipController {
	pendingTaskIDs = $state<string[]>([]);

	private loadFlow: LoadFlow = async () => false;
	private weekCode: () => string = () => '';
	private fallbackMessage: () => string = () => '';
	private setErrorMessage: (message: string) => void = () => {};

	sync(input: FlowTaskRelationshipControllerInput): void {
		this.loadFlow = input.loadFlow;
		this.weekCode = input.weekCode;
		this.fallbackMessage = input.fallbackMessage;
		this.setErrorMessage = input.setErrorMessage;
	}

	setParent = async (taskID: string, parentTaskID?: string): Promise<boolean> => {
		if (!taskID || this.pendingTaskIDs.includes(taskID)) return false;
		this.pendingTaskIDs = [...this.pendingTaskIDs, taskID];
		this.setErrorMessage('');
		try {
			await updateFlowTaskParent(taskID, parentTaskID, this.fallbackMessage());
			await this.loadFlow(this.weekCode(), { reloadState: true });
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
			await updateFlowTaskParents(uniqueTaskIDs, parentTaskID, this.fallbackMessage());
			await this.loadFlow(this.weekCode(), { reloadState: true });
			return true;
		} catch (error) {
			this.setErrorMessage(error instanceof Error ? error.message : this.fallbackMessage());
			return false;
		} finally {
			this.pendingTaskIDs = this.pendingTaskIDs.filter((pendingTaskID) => !uniqueTaskIDs.includes(pendingTaskID));
		}
	};

	isPending(task: FlowTask): boolean {
		return this.pendingTaskIDs.includes(task.id);
	}
}
