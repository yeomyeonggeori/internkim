import { getContext, setContext } from 'svelte';
import {
	fetchAttendanceWorkStatus,
	type AttendanceWorkStatus,
	type AttendanceWorkStatusPeriod
} from '../attendance-api';

export class WorkStatusState {
	payload = $state<AttendanceWorkStatus | null>(null);
	isLoading = $state(false);
	errorMessage = $state('');
	private requestSequence = 0;

	async load(period: AttendanceWorkStatusPeriod, anchor: string): Promise<void> {
		if (!anchor) return;
		const requestSequence = ++this.requestSequence;
		this.isLoading = true;
		this.errorMessage = '';
		try {
			const payload = await fetchAttendanceWorkStatus({ period, anchor });
			if (requestSequence !== this.requestSequence) return;
			this.payload = payload;
		} catch (error) {
			if (requestSequence !== this.requestSequence) return;
			this.errorMessage = error instanceof Error ? error.message : String(error);
		} finally {
			if (requestSequence === this.requestSequence) this.isLoading = false;
		}
	}
}

const workStatusStateKey = Symbol('work-status-state');

export function setWorkStatusState(state: WorkStatusState): void {
	setContext(workStatusStateKey, state);
}

export function getWorkStatusState(): WorkStatusState {
	const state = getContext<WorkStatusState | undefined>(workStatusStateKey);
	if (!state) throw new Error('WorkStatusState not provided');
	return state;
}
