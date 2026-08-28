import { getContext, setContext } from 'svelte';
import {
	attendanceWorkStatusPairFrom,
	fetchAttendanceWorkStatusPair,
	type AttendanceWorkStatus,
	type AttendanceWorkStatusPeriod,
	type AttendanceWorkStatusPair
} from '../attendance-api';
import type { SupabaseWorkStatusInputs } from '$lib/attendance/supabase-work-status';

export class WorkStatusState {
	payload = $state<AttendanceWorkStatus | null>(null);
	monthPayload = $state<AttendanceWorkStatus | null>(null);
	isLoading = $state(false);
	errorMessage = $state('');
	private requestSequence = 0;
	private rows: SupabaseWorkStatusInputs | undefined;
	private rowsAsOf: unknown;

	async load(
		period: AttendanceWorkStatusPeriod,
		anchor: string,
		rowsAsOf?: unknown
	): Promise<void> {
		if (!anchor) return;
		const asked = { period, anchor };
		const month = { period: 'month' as const, anchor };
		const reused =
			this.rows && rowsAsOf !== undefined && rowsAsOf === this.rowsAsOf
				? attendanceWorkStatusPairFrom(this.rows, asked, month)
				: undefined;
		if (reused) {
			++this.requestSequence;
			this.isLoading = false;
			this.errorMessage = '';
			this.adopt(reused, rowsAsOf);
			return;
		}
		const requestSequence = ++this.requestSequence;
		this.isLoading = true;
		this.errorMessage = '';
		try {
			const answered = await fetchAttendanceWorkStatusPair(asked, month);
			if (requestSequence !== this.requestSequence) return;
			this.adopt(answered, rowsAsOf);
		} catch (error) {
			if (requestSequence !== this.requestSequence) return;
			this.errorMessage = error instanceof Error ? error.message : String(error);
		} finally {
			if (requestSequence === this.requestSequence) this.isLoading = false;
		}
	}

	private adopt(answered: AttendanceWorkStatusPair, rowsAsOf: unknown): void {
		this.payload = answered.period;
		this.monthPayload = answered.month;
		this.rows = answered.rows;
		this.rowsAsOf = answered.rows ? rowsAsOf : undefined;
	}
}

const workStatusStateKey = Symbol('work-status-state');

export function setWorkStatusState(state: WorkStatusState): void {
	setContext(workStatusStateKey, state);
}

export function getWorkStatusState(): WorkStatusState {
	const state = optionalWorkStatusState();
	if (!state) throw new Error('WorkStatusState not provided');
	return state;
}

export function optionalWorkStatusState(): WorkStatusState | undefined {
	return getContext<WorkStatusState | undefined>(workStatusStateKey);
}
