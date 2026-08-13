import type { FlowTask } from './flow-types';
import { flowBusinessLabel } from './flow-task-workspace-model';

export type FlowTaskBoardCardDisplay = {
	participantNames: string[];
	participantIDs: string[];
	businessLabel: string;
	metadataLabels: string[];
};

export function buildFlowTaskBoardCardDisplay(task: FlowTask, emptyBusinessLabel = '기타'): FlowTaskBoardCardDisplay {
	return {
		participantNames: task.participantNames,
		participantIDs: task.participantIDs,
		businessLabel: flowBusinessLabel(task.business, emptyBusinessLabel),
		metadataLabels: buildMetadataLabels(task)
	};
}

function buildMetadataLabels(task: FlowTask): string[] {
	return [task.type].filter((label): label is string => Boolean(label));
}
