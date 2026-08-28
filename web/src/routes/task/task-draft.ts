import type { TaskDefinitions, TaskMember, Task } from './task-types';
import { compatibilityOwnerOf } from '$lib/task/central-task';
import { isCentralTaskSource } from './task-source';

type TaskParticipantIdentity = {
	id: string;
	name: string;
};

type TaskParticipantSelection = Pick<Task, 'ownerID' | 'ownerName' | 'participantIDs' | 'participantNames'>;

export function createTaskDraft(owner: TaskMember, definitions: TaskDefinitions, weekCode: string): Task {
	return {
		id: '',
		ownerID: owner.id,
		ownerName: owner.name,
		participantIDs: [owner.id],
		participantNames: [owner.name],
		business: definitions.categories[0] ?? '',
		type: definitions.types[0] ?? '',
		content: '',
			size: 'M',
		status: 'planned',
		statusRank: 0,
		weekCode,
		};
}

export function cloneTask(task: Task): Task {
	return {
		...task,
		participantIDs: [...task.participantIDs],
		participantNames: [...task.participantNames]
	};
}

export function toggleTaskParticipantID(task: Task, memberID: string, canRemoveParticipant: boolean): string[] {
	if (!task.participantIDs.includes(memberID)) return [...task.participantIDs, memberID];
	if (!canRemoveParticipant) return [...task.participantIDs];
	return task.participantIDs.filter((participantID) => participantID !== memberID);
}

export function defaultTaskOwner(members: TaskMember[], currentUserEmail: string, activeMemberID: string): TaskMember | undefined {
	if (activeMemberID) {
		const activeMember = members.find((member) => member.id === activeMemberID);
		if (activeMember) return activeMember;
	}
	return members.find((member) => member.email === currentUserEmail) ?? members[0];
}

export function participantSelectionFromIDs(
	memberIDs: string[],
	members: TaskMember[]
): TaskParticipantIdentity[] {
	const memberByID = new Map(members.map((member) => [member.id, member]));
	const participants: TaskParticipantIdentity[] = [];
	for (const memberID of memberIDs) {
		const member = memberByID.get(memberID);
		if (member && !participants.some((participant) => participant.id === member.id)) {
			participants.push({ id: member.id, name: member.name });
		}
	}
	return participants;
}

export function updateTaskParticipantIDs(task: Task, members: TaskMember[], memberIDs: string[], source: string): Task {
	const participants = participantSelectionFromIDs(memberIDs, members);
	const selection = participantSelectionForSource(task, participants, source);
	return {
		...task,
		...selection
	};
}

export function removeTaskParticipant(task: Task, memberID: string, source: string): Task {
	if (!isCentralTaskSource(source) && memberID === task.ownerID) return task;
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
	task: Task,
	participants: TaskParticipantIdentity[],
	source: string
): TaskParticipantSelection {
	if (isCentralTaskSource(source)) return centralParticipantSelection(participants);
	return deviceParticipantSelection(task, participants);
}

function centralParticipantSelection(participants: TaskParticipantIdentity[]): TaskParticipantSelection {
	const owner = compatibilityOwnerOf(participants);
	return participantSelection(owner, participants);
}

function deviceParticipantSelection(task: Task, participants: TaskParticipantIdentity[]): TaskParticipantSelection {
	const owner = deviceTaskOwner(task, participants);
	const orderedParticipants = owner.id
		? [owner, ...participants.filter((participant) => participant.id !== owner.id)]
		: participants;
	return participantSelection(owner, orderedParticipants);
}

function deviceTaskOwner(task: Task, participants: TaskParticipantIdentity[]): TaskParticipantIdentity {
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
	owner: TaskParticipantIdentity,
	participants: TaskParticipantIdentity[]
): TaskParticipantSelection {
	return {
		ownerID: owner.id,
		ownerName: owner.name,
		participantIDs: participants.map((participant) => participant.id),
		participantNames: participants.map((participant) => participant.name)
	};
}
