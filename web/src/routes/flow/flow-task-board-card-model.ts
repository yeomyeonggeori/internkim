import type { FlowTask } from './flow-types';

export type FlowTaskBoardCardDisplay = {
	ownerName: string;
	participantNames: string[];
	metadataLabels: string[];
	dateLabel: string;
};

export function buildFlowTaskBoardCardDisplay(task: FlowTask): FlowTaskBoardCardDisplay {
	return {
		ownerName: task.ownerName,
		participantNames: participantNamesWithoutOwner(task),
		metadataLabels: buildMetadataLabels(task),
		dateLabel: [task.startDate, task.endDate].filter(Boolean).join(' - ')
	};
}

function participantNamesWithoutOwner(task: FlowTask): string[] {
	return task.participantNames.filter((_, index) => task.participantIDs[index] !== task.ownerID);
}

function buildMetadataLabels(task: FlowTask): string[] {
	const labels = [task.business, task.type].filter((label): label is string => Boolean(label));
	if (task.flag > 0) return [...labels, `F ${task.flag}`];
	return labels;
}
