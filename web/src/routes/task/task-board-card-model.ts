import type { Task } from './task-types';
import { taskBusinessLabel } from './task-workspace-model';

export type TaskBoardCardDisplay = {
	participantNames: string[];
	participantIDs: string[];
	businessLabel: string;
	metadataLabels: string[];
};

export function buildTaskBoardCardDisplay(task: Task, emptyBusinessLabel = '기타'): TaskBoardCardDisplay {
	return {
		participantNames: task.participantNames,
		participantIDs: task.participantIDs,
		businessLabel: taskBusinessLabel(task.business, emptyBusinessLabel),
		metadataLabels: buildMetadataLabels(task)
	};
}

function buildMetadataLabels(task: Task): string[] {
	return [task.type].filter((label): label is string => Boolean(label));
}
