import { getContext, setContext } from 'svelte';
import {
	attendanceWorkStatusPairFrom,
	fetchAttendanceWorkStatusPair,
	type AttendanceWorkStatus,
	type AttendanceWorkStatusPeriod,
	type AttendanceWorkStatusPair
} from '../attendance-api';
import type { SupabaseWorkStatusInputs } from '$lib/attendance/supabase-work-status';
import type { AttendanceWriteEvent } from '$lib/attendance/attendance-write';
import { readCachedWorkStatusRows, writeCachedWorkStatusRows } from './work-status-cache';
import type { AttendanceSummaryRecords } from '$lib/attendance/attendance-summary-records';

export class WorkStatusState {
	payload = $state<AttendanceWorkStatus | null>(null);
	monthPayload = $state<AttendanceWorkStatus | null>(null);
	isLoading = $state(false);
	errorMessage = $state('');
	private requestSequence = 0;
	private rows: SupabaseWorkStatusInputs | undefined;
	private rowsAsOf: unknown;
	private periodRequest: { period: AttendanceWorkStatusPeriod; anchor: string } | undefined;
	private monthRequest: { period: 'month'; anchor: string } | undefined;
	private savedAttendanceEvents: AttendanceWriteEvent[] = [];

	constructor(private readonly cacheScope = '') {}

	async load(
		period: AttendanceWorkStatusPeriod,
		anchor: string,
		rowsAsOf?: unknown,
		summaryRecords?: AttendanceSummaryRecords
	): Promise<void> {
		if (!anchor) return;
		const asked = { period, anchor };
		const month = { period: 'month' as const, anchor };
		this.periodRequest = asked;
		this.monthRequest = month;
		const savedEventCount = this.savedAttendanceEvents.length;
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
		if (!this.payload) this.adoptCachedRows(asked, month);
		const requestSequence = ++this.requestSequence;
		this.isLoading = true;
		this.errorMessage = '';
		try {
			const answered = await fetchAttendanceWorkStatusPair(asked, month, summaryRecords);
			if (requestSequence !== this.requestSequence) return;
			const rows = answered.rows;
			const eventsDuringLoad = this.savedAttendanceEvents.slice(savedEventCount);
			if (!rows || eventsDuringLoad.length === 0) {
				this.adopt(answered, rowsAsOf);
				return;
			}
			const mergedRows = applySavedAttendanceEvents(rows, eventsDuringLoad);
			const merged = attendanceWorkStatusPairFrom(mergedRows, asked, month);
			this.adopt(merged ?? { ...answered, rows: mergedRows }, rowsAsOf);
		} catch (error) {
			if (requestSequence !== this.requestSequence) return;
			this.errorMessage = error instanceof Error ? error.message : String(error);
		} finally {
			if (requestSequence === this.requestSequence) this.isLoading = false;
		}
	}

	applyAttendanceEvent(event: AttendanceWriteEvent): void {
		this.savedAttendanceEvents = [...this.savedAttendanceEvents, event];
		if (!this.rows || !this.periodRequest || !this.monthRequest) {
			return;
		}
		this.rows = applySavedAttendanceEvents(this.rows, [event]);
		const pair = attendanceWorkStatusPairFrom(this.rows, this.periodRequest, this.monthRequest);
		if (!pair) return;
		this.adopt(pair, this.rowsAsOf);
	}

	private adoptCachedRows(
		period: { period: AttendanceWorkStatusPeriod; anchor: string },
		month: { period: 'month'; anchor: string }
	): void {
		const cachedRows = readCachedWorkStatusRows(this.cacheScope);
		if (!cachedRows) return;
		const cached = attendanceWorkStatusPairFrom(cachedRows, period, month);
		if (cached) this.adopt(cached, undefined);
	}

	private adopt(answered: AttendanceWorkStatusPair, rowsAsOf: unknown): void {
		this.payload = answered.period;
		this.monthPayload = answered.month;
		this.rows = answered.rows;
		this.rowsAsOf = answered.rows ? rowsAsOf : undefined;
		if (answered.rows && rowsAsOf !== undefined) writeCachedWorkStatusRows(answered.rows, this.cacheScope);
	}
}

function applySavedAttendanceEvents(
	rows: SupabaseWorkStatusInputs,
	events: AttendanceWriteEvent[]
): SupabaseWorkStatusInputs {
	const knownEventIDs = new Set(rows.attendance.flatMap((event) => event.id ? [event.id] : []));
	const additions = events
		.filter((event) => !knownEventIDs.has(event.id))
		.map((event) => ({
			id: event.id,
			member_id: event.personID,
			kind: event.kind,
			occurred_at: event.occurredAt,
			location: event.location
		}));
	return additions.length === 0 ? rows : { ...rows, attendance: [...rows.attendance, ...additions] };
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
