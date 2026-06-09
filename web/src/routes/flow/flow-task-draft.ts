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
