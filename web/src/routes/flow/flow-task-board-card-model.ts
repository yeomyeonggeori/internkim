import type { FlowTask } from './flow-types';
import { flowBusinessLabel } from './flow-task-workspace-model';

export type FlowTaskBoardCardDisplay = {
	ownerName: string;
	participantNames: string[];
	participantIDs: string[];
	metadataLabels: string[];
	dateLabel: string;
};

export function buildFlowTaskBoardCardDisplay(task: FlowTask, emptyBusinessLabel = '기타'): FlowTaskBoardCardDisplay {
	return {
		ownerName: task.ownerName,
		participantNames: participantNamesWithoutOwner(task),
		participantIDs: participantIDsWithoutOwner(task),
		metadataLabels: buildMetadataLabels(task, emptyBusinessLabel),
		dateLabel: [task.startDate, task.endDate].filter(Boolean).join(' - ')
	};
}

function participantNamesWithoutOwner(task: FlowTask): string[] {
	return task.participantNames.filter((_, index) => task.participantIDs[index] !== task.ownerID);
}

function participantIDsWithoutOwner(task: FlowTask): string[] {
	return task.participantIDs.filter((participantID) => participantID !== task.ownerID);
}

function buildMetadataLabels(task: FlowTask, emptyBusinessLabel: string): string[] {
	return [flowBusinessLabel(task.business, emptyBusinessLabel), task.type].filter((label): label is string => Boolean(label));
}
