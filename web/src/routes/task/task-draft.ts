import type { TaskDefinitions, TaskMember, Task } from './task-types';
import { compatibilityOwnerOf } from '$lib/task/central-task';

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
		business: null,
		type: null,
		content: '',
		size: '',
		status: 'planned',
		weekCode
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

export function updateTaskParticipantIDs(task: Task, members: TaskMember[], memberIDs: string[]): Task {
	return {
		...task,
		...participantSelectionOf(participantSelectionFromIDs(memberIDs, members))
	};
}

export function removeTaskParticipant(task: Task, memberID: string): Task {
	const participants = task.participantIDs
		.map((participantID, index) => ({
			id: participantID,
			name: task.participantNames[index] ?? ''
		}))
		.filter((participant) => participant.id !== memberID);
	return {
		...task,
		...participantSelectionOf(participants)
	};
}

function participantSelectionOf(participants: TaskParticipantIdentity[]): TaskParticipantSelection {
	const owner = compatibilityOwnerOf(participants);
	return {
		ownerID: owner.id,
		ownerName: owner.name,
		participantIDs: participants.map((participant) => participant.id),
		participantNames: participants.map((participant) => participant.name)
	};
}
