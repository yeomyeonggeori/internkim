import type { Task } from '../../routes/task/task-types';
import { taskStatus, type TaskStatus } from '../../routes/task/task-status';

export type CentralTaskStatus = TaskStatus;

type CentralRequesterIdentity = {
	name: string | null;
	email: string | null;
};

export type CentralTaskRow = {
	id: string;
	parent_task_id: string | null;
	title: string;
	status: string;
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
	requester: CentralRequesterIdentity | CentralRequesterIdentity[] | null;
	task_participant: { member_id: string }[];
};

const centralStatuses: readonly CentralTaskStatus[] = [
	taskStatus.requested,
	taskStatus.planned,
	taskStatus.inProgress,
	taskStatus.completed,
	taskStatus.paused,
	taskStatus.rejected,
	taskStatus.stopped
];

export const centralTaskSelection =
	'id, parent_task_id, title, status, note, business, type, size, starts_at, ends_at, due_at, is_event, updated_at, requester_id, requester:member!task_requester_id_fkey (name, email), task_participant (member_id)';

export const centralTaskStatusOptions: string[] = [...centralStatuses];

export function centralTaskFromRow(
	row: CentralTaskRow,
	nameByID: Map<string, string>,
	dayOf: (instant: string | null) => string | undefined,
	weekCodeOfDay: (day: string) => string = () => ''
): Task {
	const participants = row.task_participant.map(({ member_id }) => ({
		id: member_id,
		name: nameByID.get(member_id) ?? ''
	}));
	const owner = compatibilityOwnerOf(participants);
	const endDate = dayOf(row.ends_at ?? row.due_at);
	const requesterID = row.requester_id ?? '';
	const requester = Array.isArray(row.requester) ? row.requester[0] : row.requester;
	return {
		id: row.id,
		parentTaskID: row.parent_task_id ?? undefined,
		ownerID: owner.id,
		ownerName: owner.name,
		participantIDs: participants.map(({ id }) => id),
		participantNames: participants.map(({ name }) => name),
		requesterID,
		requesterName: nameByID.get(requesterID) || requester?.name || requester?.email || '',
		business: row.business,
		type: row.type,
		content: row.title,
		size: row.size ?? '',
		status: centralStatusWord(row.status),
		statusRank: 0,
		startDate: dayOf(row.starts_at),
		endDate,
		weekCode: endDate ? weekCodeOfDay(endDate) : '',
		isEvent: row.is_event
	};
}

export function centralTaskWriteFields(
	task: Task,
	operation: 'insert' | 'update'
): Record<string, unknown> {
	return {
		title: task.content || '(제목 없음)',
		...(operation === 'insert' ? { parent_task_id: task.parentTaskID ?? null } : {}),
		status: centralStatusFromWord(task.status),
		note: null,
		business: task.business,
		type: task.type,
		size: task.size || null,
		...(operation === 'insert' ? { requester_id: task.requesterID || null } : {}),
		...(task.isEvent ? {} : { starts_at: instantOf(task.startDate), ends_at: instantOf(task.endDate) })
	};
}

export function centralStatusFromWord(status: string): CentralTaskStatus {
	if (!centralStatuses.includes(status as CentralTaskStatus)) {
		throw new Error(`unsupported task status: ${status}`);
	}
	return status as CentralTaskStatus;
}

export function centralStatusWord(status: string): string {
	return centralStatusFromWord(status);
}

export function compatibilityOwnerOf(participants: { id: string; name: string }[]): { id: string; name: string } {
	if (participants.length !== 1) return { id: '', name: '' };
	return participants[0];
}

function instantOf(day: string | undefined): string | null {
	return day ? new Date(`${day}T00:00:00Z`).toISOString() : null;
}
