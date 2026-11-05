import type { TaskRow } from './tasks';

export type TaskChange = { taskID: string; moved: boolean; askedMemberIDs: string[] };

export type TaskChangeAnnouncer = (change: TaskChange) => Promise<void>;

type TaskStanding = Pick<TaskRow, 'id' | 'status' | 'task_participant'>;

export function taskChangeOf(before: TaskStanding | null, after: TaskStanding, moverID: string): TaskChange {
	const participantsBefore = new Set((before?.task_participant ?? []).map((participant) => participant.member_id));
	return {
		taskID: after.id,
		moved: before !== null && before.status !== after.status,
		askedMemberIDs: after.task_participant
			.map((participant) => participant.member_id)
			.filter((memberID) => memberID !== moverID && !participantsBefore.has(memberID))
	};
}

export function changesAnythingAnnounced(change: TaskChange): boolean {
	return change.moved || change.askedMemberIDs.length > 0;
}
