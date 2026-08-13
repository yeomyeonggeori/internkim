import type { FlowDefinitions, FlowMember, FlowTask } from './flow-types';
import { compatibilityOwnerOf } from '$lib/flow/central-flow-task';

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

export function defaultFlowTaskOwner(members: FlowMember[], currentUserEmail: string, activeMemberID: string): FlowMember | undefined {
	if (activeMemberID) {
		const activeMember = members.find((member) => member.id === activeMemberID);
		if (activeMember) return activeMember;
	}
	return members.find((member) => member.email === currentUserEmail) ?? members[0];
}

export function participantSelectionFromNames(
	names: string[],
	members: FlowMember[]
): Pick<FlowTask, 'ownerID' | 'ownerName' | 'participantIDs' | 'participantNames'> {
	const memberByName = new Map(members.map((member) => [member.name, member]));
	const ordered: FlowMember[] = [];
	for (const name of names) {
		const candidate = memberByName.get(name);
		if (candidate && !ordered.some((member) => member.id === candidate.id)) {
			ordered.push(candidate);
		}
	}
	const owner = compatibilityOwnerOf(ordered);
	return {
		ownerID: owner.id,
		ownerName: owner.name,
		participantIDs: ordered.map((member) => member.id),
		participantNames: ordered.map((member) => member.name)
	};
}

export function participantSelectionFromIDs(
	memberIDs: string[],
	members: FlowMember[]
): Pick<FlowTask, 'ownerID' | 'ownerName' | 'participantIDs' | 'participantNames'> {
	const memberByID = new Map(members.map((member) => [member.id, member]));
	const participants: FlowMember[] = [];
	for (const memberID of memberIDs) {
		const member = memberByID.get(memberID);
		if (member && !participants.some((participant) => participant.id === member.id)) participants.push(member);
	}
	const owner = compatibilityOwnerOf(participants);
	return {
		ownerID: owner.id,
		ownerName: owner.name,
		participantIDs: participants.map((participant) => participant.id),
		participantNames: participants.map((participant) => participant.name)
	};
}

export function updateFlowTaskParticipantNames(task: FlowTask, members: FlowMember[], names: string[]): FlowTask {
	const selection = participantSelectionFromNames(names, members);
	return {
		...task,
		ownerID: selection.ownerID,
		ownerName: selection.ownerName,
		participantIDs: selection.participantIDs,
		participantNames: selection.participantNames
	};
}

export function updateFlowTaskParticipantIDs(task: FlowTask, members: FlowMember[], memberIDs: string[]): FlowTask {
	const selection = participantSelectionFromIDs(memberIDs, members);
	return {
		...task,
		...selection
	};
}

export function removeFlowTaskParticipant(task: FlowTask, memberID: string): FlowTask {
	const participants = task.participantIDs
		.map((participantID, index) => ({
			id: participantID,
			name: task.participantNames[index] ?? ''
		}))
		.filter((participant) => participant.id !== memberID);
	const owner = compatibilityOwnerOf(participants);
	return {
		...task,
		ownerID: owner.id,
		ownerName: owner.name,
		participantIDs: participants.map((participant) => participant.id),
		participantNames: participants.map((participant) => participant.name)
	};
}
