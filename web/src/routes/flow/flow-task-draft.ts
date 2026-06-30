import type { FlowDefinitions, FlowMember, FlowTask } from './flow-types';

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
	members: FlowMember[],
	ownerID: string
): Pick<FlowTask, 'participantIDs' | 'participantNames'> {
	const owner = members.find((member) => member.id === ownerID);
	const memberByName = new Map(members.map((member) => [member.name, member]));
	const ordered: FlowMember[] = [];
	if (owner) ordered.push(owner);
	for (const name of names) {
		const candidate = memberByName.get(name);
		if (candidate && !ordered.some((member) => member.id === candidate.id)) {
			ordered.push(candidate);
		}
	}
	return {
		participantIDs: ordered.map((member) => member.id),
		participantNames: ordered.map((member) => member.name)
	};
}

export function updateFlowTaskOwner(task: FlowTask, members: FlowMember[], ownerID: string): FlowTask {
	const owner = members.find((member) => member.id === ownerID);
	if (!owner) return task;
	const memberByID = new Map(members.map((member) => [member.id, member]));
	const participantIDs = Array.from(new Set([owner.id, ...task.participantIDs]));
	const participants = participantIDs
		.map((participantID) => memberByID.get(participantID))
		.filter((member): member is FlowMember => Boolean(member));
	return {
		...task,
		ownerID: owner.id,
		ownerName: owner.name,
		participantIDs: participants.map((participant) => participant.id),
		participantNames: participants.map((participant) => participant.name)
	};
}

export function updateFlowTaskParticipantNames(task: FlowTask, members: FlowMember[], names: string[]): FlowTask {
	const selection = participantSelectionFromNames(names, members, task.ownerID);
	return {
		...task,
		participantIDs: selection.participantIDs,
		participantNames: selection.participantNames
	};
}

export function removeFlowTaskParticipant(task: FlowTask, memberID: string): FlowTask {
	const participants = task.participantIDs
		.map((participantID, index) => ({
			id: participantID,
			name: task.participantNames[index] ?? ''
		}))
		.filter((participant) => participant.id !== memberID);
	return {
		...task,
		participantIDs: participants.map((participant) => participant.id),
		participantNames: participants.map((participant) => participant.name)
	};
}
