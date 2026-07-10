import { getContext, setContext } from 'svelte';
import {
	createAttendanceAbsence,
	type CreateAttendanceAbsenceRequest,
	deleteAttendanceAbsence,
	fetchAttendanceSummary,
	toggleAttendanceOnServer,
	updateAttendanceEvent,
	type UpdateAttendanceEventRequest
} from './attendance-api';
import { readPersistedAttendanceFilters, writePersistedAttendanceFilters } from './attendance-storage';
import { currentMonthInTimeZone } from './shared/attendance-date';

export type AttendanceKind = 'clock_in' | 'clock_out';
export type AttendanceAbsenceKind = 'leave' | 'other';
export type ChartMode = 'day' | 'week' | 'month';
export type AttendancePresence = 'online' | 'away' | 'offline' | 'dnd';

export type AttendanceLocation = {
	id: string;
	name: string;
	color: string;
	isDefault: boolean;
};

export type AttendanceEvent = {
	id: string;
	mattermostUserID: string;
	mattermostUsername: string;
	email: string;
	displayName: string;
	kind: AttendanceKind;
	occurredAt: string;
	localDate: string;
	localTime: string;
	timeZoneAtEvent: string;
	source: string;
	resultPostID: string;
	locationID?: string;
	locationName?: string;
	canceledAt?: string;
	cancelReason?: string;
	sourceMessage?: string;
	confidence?: number;
	parsedAs?: { kind?: AttendanceKind; locationID?: string };
	overriddenBy?: string;
	overriddenAt?: string;
	overrideReason?: string;
	originalOccurredAt?: string;
	originalLocalDate?: string;
	originalLocalTime?: string;
	originalLocationID?: string;
	originalLocationName?: string;
	overrideHistory?: AttendanceEventOverride[];
	manualEntry?: boolean;
};

export type AttendanceEventOverride = {
	id: string;
	eventID: string;
	editedBy: string;
	editedAt: string;
	reason: string;
	originalOccurredAt: string;
	originalLocalDate: string;
	originalLocalTime: string;
	originalLocationID: string;
	originalLocationName: string;
	overrideOccurredAt: string;
	overrideLocalDate: string;
	overrideLocalTime: string;
	overrideLocationID: string;
	overrideLocationName: string;
};

export type AttendanceAbsence = {
	id: string;
	rangeID?: string;
	email: string;
	kind: AttendanceAbsenceKind;
	labelKey: AttendanceAbsenceKind;
	date: string;
	startDate?: string;
	endDate?: string;
	reason?: string;
	createdBy?: string;
	createdAt: string;
	canceledAt?: string;
	isRangeStart?: boolean;
	isRangeEnd?: boolean;
	isChunkStart?: boolean;
	isChunkEnd?: boolean;
};

export type AttendanceMember = {
	email: string;
	displayName: string;
	image?: string;
	mattermostUsername: string;
};

export type AttendanceSummary = {
	month: string;
	currentUserEmail: string;
	isAdmin: boolean;
	timeZone: string;
	events: AttendanceEvent[];
	absences: AttendanceAbsence[];
	members: AttendanceMember[];
	todayStatus: string;
	locations: AttendanceLocation[];
	teamViewVisibleToAll: boolean;
	teamViewBlocked: boolean;
	presences?: Record<string, AttendancePresence>;
};

export class AttendanceState {
	summary = $state<AttendanceSummary | null>(null);
	currentMonthSummary = $state<AttendanceSummary | null>(null);
	selectedMonth = $state<string>('');
	chartMode = $state<ChartMode>('day');
	selectedDate = $state<string>('');
	isLoading = $state<boolean>(false);
	errorMessage = $state<string>('');

	private loadFailedMessage: string;

	constructor(loadFailedMessage: string) {
		this.loadFailedMessage = loadFailedMessage;
		const persisted = readPersistedAttendanceFilters();
		if (persisted.selectedMonth) this.selectedMonth = persisted.selectedMonth;
		if (persisted.chartMode) this.chartMode = persisted.chartMode;
	}

	persistFilters() {
		writePersistedAttendanceFilters({
			selectedMonth: this.selectedMonth,
			chartMode: this.chartMode
		});
	}

	async load() {
		this.isLoading = true;
		this.errorMessage = '';
		try {
			const next = await this.fetchSummaryForMonth(this.selectedMonth);
			this.summary = next;
			this.selectedMonth = next.month;
			await this.refreshCurrentMonthSnapshot(next);
		} catch (error) {
			this.errorMessage = error instanceof Error ? error.message : this.loadFailedMessage;
			this.summary = null;
			this.currentMonthSummary = null;
		} finally {
			this.isLoading = false;
		}
	}

	private fetchSummaryForMonth(month: string): Promise<AttendanceSummary> {
		return fetchAttendanceSummary({ month });
	}

	private async refreshCurrentMonthSnapshot(filteredSummary: AttendanceSummary) {
		const currentMonth = currentMonthInTimeZone(filteredSummary.timeZone);
		if (filteredSummary.month === currentMonth) {
			this.currentMonthSummary = filteredSummary;
			return;
		}
		try {
			this.currentMonthSummary = await this.fetchSummaryForMonth(currentMonth);
		} catch {
			this.currentMonthSummary = null;
		}
	}

	async toggleAttendance(kind?: AttendanceKind, locationID?: string) {
		await toggleAttendanceOnServer(kind, locationID);
		await this.load();
	}

	async createAbsence(request: CreateAttendanceAbsenceRequest) {
		const absences = await createAttendanceAbsence(request);
		await this.load();
		return absences;
	}

	async deleteAbsence(absenceID: string) {
		await deleteAttendanceAbsence(absenceID);
		await this.load();
	}

	async updateEvent(eventID: string, request: UpdateAttendanceEventRequest) {
		await updateAttendanceEvent(eventID, request);
		await this.load();
	}

	async updateEvents(updates: { eventID: string; request: UpdateAttendanceEventRequest }[]) {
		await Promise.all(updates.map((update) => updateAttendanceEvent(update.eventID, update.request)));
		await this.load();
	}
}

const KEY = Symbol('attendance-state');
export const setAttendanceState = (s: AttendanceState) => setContext(KEY, s);
export const getAttendanceState = (): AttendanceState => {
	const s = getContext<AttendanceState | undefined>(KEY);
	if (!s) throw new Error('AttendanceState not provided — must wrap in attendance/+layout.svelte');
	return s;
};
