import { taskStatus, type TaskStatus } from '../../routes/task/task-status';

export { taskStatus };
export type CentralTaskStatus = TaskStatus;

const centralStatuses: readonly CentralTaskStatus[] = [
	taskStatus.requested,
	taskStatus.planned,
	taskStatus.inProgress,
	taskStatus.completed,
	taskStatus.paused,
	taskStatus.rejected,
	taskStatus.stopped
];

export const centralTaskStatusOptions: string[] = [...centralStatuses];

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

