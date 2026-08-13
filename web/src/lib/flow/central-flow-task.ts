import type { FlowTask } from '../../routes/flow/flow-types';
import { flowStatus } from '../../routes/flow/flow-status';

export type CentralTaskStatus =
	| 'requested'
	| 'todo'
	| 'in_progress'
	| 'paused'
	| 'cancelled'
	| 'rejected'
	| 'done';

export type CentralFlowTaskRow = {
	id: string;
	title: string;
	status: CentralTaskStatus;
	note: string | null;
	business: string | null;
	type: string | null;
	size: string | null;
	is_event: boolean;
	starts_at: string | null;
	ends_at: string | null;
	due_at: string | null;
	updated_at: string;
	requester_id: string | null;
	task_participant: { member_id: string }[];
};

const centralStatusWords: Record<CentralTaskStatus, string> = {
	requested: flowStatus.requested,
	todo: flowStatus.planned,
	in_progress: flowStatus.inProgress,
	done: flowStatus.completed,
	paused: flowStatus.paused,
	rejected: flowStatus.rejected,
	cancelled: flowStatus.stopped
};

const centralStatusByWord = new Map<string, CentralTaskStatus>([
	[flowStatus.requested, 'requested'],
	[flowStatus.planned, 'todo'],
	[flowStatus.inProgress, 'in_progress'],
	[flowStatus.completed, 'done'],
	[flowStatus.paused, 'paused'],
	[flowStatus.rejected, 'rejected'],
	[flowStatus.stopped, 'cancelled']
]);

export const centralFlowTaskSelection =
	'id, title, status, note, business, type, size, starts_at, ends_at, due_at, is_event, updated_at, requester_id, task_participant (member_id)';

export const centralFlowStatusOptions = Object.values(centralStatusWords);

export function centralFlowTaskFromRow(
	row: CentralFlowTaskRow,
	nameByID: Map<string, string>,
	dayOf: (instant: string | null) => string | undefined,
	weekCodeOfDay: (day: string) => string = () => ''
): FlowTask {
	const participants = row.task_participant.map(({ member_id }) => ({
		id: member_id,
		name: nameByID.get(member_id) ?? ''
	}));
	const owner = compatibilityOwnerOf(participants);
	const endDate = dayOf(row.ends_at ?? row.due_at);
	const requesterID = row.requester_id ?? '';
	return {
		id: row.id,
		ownerID: owner.id,
		ownerName: owner.name,
		participantIDs: participants.map(({ id }) => id),
		participantNames: participants.map(({ name }) => name),
		requesterID,
		requesterName: nameByID.get(requesterID) ?? '',
		business: row.business ?? '',
		type: row.type ?? '',
		content: row.title,
		goal: row.note ?? '',
		size: row.size ?? '',
		status: centralStatusWords[row.status],
		statusRank: 0,
		startDate: dayOf(row.starts_at),
		endDate,
		weekCode: endDate ? weekCodeOfDay(endDate) : '',
		flag: 0,
		isEvent: row.is_event
	};
}

export function centralFlowTaskWriteFields(
	task: FlowTask,
	operation: 'insert' | 'update'
): Record<string, unknown> {
	return {
		title: task.content || task.goal || '(제목 없음)',
		status: centralStatusFromWord(task.status),
		note: task.goal || null,
		business: task.business || null,
		type: task.type || null,
		size: task.size || null,
		...(operation === 'insert' ? { requester_id: task.requesterID || null } : {}),
		...(task.isEvent ? {} : { starts_at: instantOf(task.startDate), ends_at: instantOf(task.endDate) })
	};
}

export function centralStatusFromWord(status: string): CentralTaskStatus {
	const centralStatus = centralStatusByWord.get(status);
	if (!centralStatus) throw new Error(`unsupported Flow task status: ${status}`);
	return centralStatus;
}

export function compatibilityOwnerOf(participants: { id: string; name: string }[]): { id: string; name: string } {
	if (participants.length !== 1) return { id: '', name: '' };
	return participants[0];
}

function instantOf(day: string | undefined): string | null {
	return day ? new Date(`${day}T00:00:00Z`).toISOString() : null;
}
