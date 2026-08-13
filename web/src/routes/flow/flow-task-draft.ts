import type { FlowDefinitions, FlowMember, FlowTask } from './flow-types';
import { compatibilityOwnerOf } from '$lib/flow/central-flow-task';
import { isCentralFlowSource } from './flow-source';

type FlowTaskParticipantIdentity = {
	id: string;
	name: string;
};

type FlowTaskParticipantSelection = Pick<FlowTask, 'ownerID' | 'ownerName' | 'participantIDs' | 'participantNames'>;

export function createFlowTaskDraft(owner: FlowMember, definitions: FlowDefinitions, weekCode: string): FlowTask {
	return {
		id: '',
		ownerID: owner.id,
		ownerName: owner.name,
		participantIDs: [owner.id],
		participantNames: [owner.name],
		business: definitions.categories[0] ?? '',
		type: definitions.types[0] ?? '',
		content: '',
		goal: '',
		size: 'M',
		status: '예정',
		statusRank: 0,
		weekCode,
		flag: 0
	};
}

export function cloneFlowTask(task: FlowTask): FlowTask {
	return {
		...task,
		participantIDs: [...task.participantIDs],
		participantNames: [...task.participantNames]
	};
}

export function toggleFlowTaskParticipantID(task: FlowTask, memberID: string, canRemoveParticipant: boolean): string[] {
	if (!task.participantIDs.includes(memberID)) return [...task.participantIDs, memberID];
	if (!canRemoveParticipant) return [...task.participantIDs];
	return task.participantIDs.filter((participantID) => participantID !== memberID);
}

export function defaultFlowTaskOwner(members: FlowMember[], currentUserEmail: string, activeMemberID: string): FlowMember | undefined {
	if (activeMemberID) {
		const activeMember = members.find((member) => member.id === activeMemberID);
		if (activeMember) return activeMember;
	}
	return members.find((member) => member.email === currentUserEmail) ?? members[0];
}

export function participantSelectionFromIDs(
	memberIDs: string[],
	members: FlowMember[]
): FlowTaskParticipantIdentity[] {
	const memberByID = new Map(members.map((member) => [member.id, member]));
	const participants: FlowTaskParticipantIdentity[] = [];
	for (const memberID of memberIDs) {
		const member = memberByID.get(memberID);
		if (member && !participants.some((participant) => participant.id === member.id)) {
			participants.push({ id: member.id, name: member.name });
		}
	}
	return participants;
}

export function updateFlowTaskParticipantIDs(task: FlowTask, members: FlowMember[], memberIDs: string[], source: string): FlowTask {
	const participants = participantSelectionFromIDs(memberIDs, members);
	const selection = participantSelectionForSource(task, participants, source);
	return {
		...task,
		...selection
	};
}

export function removeFlowTaskParticipant(task: FlowTask, memberID: string, source: string): FlowTask {
	if (!isCentralFlowSource(source) && memberID === task.ownerID) return task;
	const participants = task.participantIDs
		.map((participantID, index) => ({
			id: participantID,
			name: task.participantNames[index] ?? ''
		}))
		.filter((participant) => participant.id !== memberID);
	return {
		...task,
		...participantSelectionForSource(task, participants, source)
	};
}

function participantSelectionForSource(
	task: FlowTask,
	participants: FlowTaskParticipantIdentity[],
	source: string
): FlowTaskParticipantSelection {
	if (isCentralFlowSource(source)) return centralParticipantSelection(participants);
	return deviceParticipantSelection(task, participants);
}

function centralParticipantSelection(participants: FlowTaskParticipantIdentity[]): FlowTaskParticipantSelection {
	const owner = compatibilityOwnerOf(participants);
	return participantSelection(owner, participants);
}

function deviceParticipantSelection(task: FlowTask, participants: FlowTaskParticipantIdentity[]): FlowTaskParticipantSelection {
	const owner = deviceTaskOwner(task, participants);
	const orderedParticipants = owner.id
		? [owner, ...participants.filter((participant) => participant.id !== owner.id)]
		: participants;
	return participantSelection(owner, orderedParticipants);
}

function deviceTaskOwner(task: FlowTask, participants: FlowTaskParticipantIdentity[]): FlowTaskParticipantIdentity {
	if (task.ownerID) {
		const selectedOwner = participants.find((participant) => participant.id === task.ownerID);
		const existingOwnerIndex = task.participantIDs.indexOf(task.ownerID);
		return {
			id: task.ownerID,
			name: selectedOwner?.name ?? task.ownerName ?? task.participantNames[existingOwnerIndex] ?? ''
		};
	}
	return participants[0] ?? { id: '', name: '' };
}

function participantSelection(
	owner: FlowTaskParticipantIdentity,
	participants: FlowTaskParticipantIdentity[]
): FlowTaskParticipantSelection {
	return {
		ownerID: owner.id,
		ownerName: owner.name,
		participantIDs: participants.map((participant) => participant.id),
		participantNames: participants.map((participant) => participant.name)
	};
}
