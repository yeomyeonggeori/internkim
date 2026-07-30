import type { FlowTask } from './flow-types';
import { flowBusinessLabel } from './flow-task-workspace-model';

export type FlowTaskBoardCardDisplay = {
	ownerName: string;
	participantNames: string[];
	participantIDs: string[];
	businessLabel: string;
	metadataLabels: string[];
};

export function buildFlowTaskBoardCardDisplay(task: FlowTask, emptyBusinessLabel = '기타'): FlowTaskBoardCardDisplay {
	return {
		ownerName: task.ownerName,
		participantNames: participantNamesWithoutOwner(task),
		participantIDs: participantIDsWithoutOwner(task),
		businessLabel: flowBusinessLabel(task.business, emptyBusinessLabel),
		metadataLabels: buildMetadataLabels(task)
	};
}

function participantNamesWithoutOwner(task: FlowTask): string[] {
	return task.participantNames.filter((_, index) => task.participantIDs[index] !== task.ownerID);
}

function participantIDsWithoutOwner(task: FlowTask): string[] {
	return task.participantIDs.filter((participantID) => participantID !== task.ownerID);
}

function buildMetadataLabels(task: FlowTask): string[] {
	return [task.type].filter((label): label is string => Boolean(label));
}
