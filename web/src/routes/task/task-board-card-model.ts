import type { Task } from './task-types';
import { taskDefinitionLabel } from './task-workspace-model';

export type TaskBoardCardDisplay = {
	participantNames: string[];
	participantIDs: string[];
	businessLabel: string;
	metadataLabels: string[];
};

export function buildTaskBoardCardDisplay(task: Task, etcLabel = '기타'): TaskBoardCardDisplay {
	return {
		participantNames: task.participantNames,
		participantIDs: task.participantIDs,
		businessLabel: taskDefinitionLabel(task.business, etcLabel),
		metadataLabels: [taskDefinitionLabel(task.type, etcLabel)]
	};
}
