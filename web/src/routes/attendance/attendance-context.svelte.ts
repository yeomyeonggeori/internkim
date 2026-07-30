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
import { clearCachedAttendanceSummaries, readCachedAttendanceSummary, writeCachedAttendanceSummary } from './attendance-summary-cache';
import { currentMonthInTimeZone } from './shared/attendance-date';
import {
	type AttendanceServerClock,
	attendanceServerTime
} from './shared/attendance-server-clock';
import { AttendanceServerClockSync } from './attendance-server-clock-sync';

const selectedMonthSummaryLoadTarget = 'selectedMonthSummary';
const currentMonthSummaryLoadTarget = 'currentMonthSummary';

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
	startTime?: string;
	endTime?: string;
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

export type AttendanceActiveLeave = {
	requestID: string;
	occurrenceID: string;
	leaveTypeID: string;
	leaveTypeName: string;
	startTime: string;
	endTime: string;
	deductionMilliDays: number;
	startAt: string;
	endAt: string;
};

export type AttendanceSummary = {
	month: string;
	serverTime?: string;
	timeZoneAuthoritative?: boolean;
	currentUserEmail: string;
	isAdmin: boolean;
	timeZone: string;
	events: AttendanceEvent[];
	absences: AttendanceAbsence[];
	members: AttendanceMember[];
	todayStatus: string;
	activeLeave?: AttendanceActiveLeave;
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
	isShowingCachedSummary = $state<boolean>(false);
	errorMessage = $state<string>('');
	serverClock = $state<AttendanceServerClock | null>(null);

	private loadFailedMessage: string;
	private readonly serverClockSync: AttendanceServerClockSync<AttendanceSummary>;
	private activeLoadCount = 0;

	constructor(loadFailedMessage: string) {
		this.loadFailedMessage = loadFailedMessage;
		this.serverClockSync = new AttendanceServerClockSync({
			requestSummary: (month) => fetchAttendanceSummary({ month }),
			applySummarySnapshot: (summary, serverClock) => {
				this.serverClock = serverClock;
				if (this.summary) {
					this.summary.serverTime = summary.serverTime;
					this.summary.timeZone = summary.timeZone;
					this.summary.timeZoneAuthoritative = summary.timeZoneAuthoritative;
					this.summary.todayStatus = summary.todayStatus;
					this.summary.activeLeave = summary.activeLeave;
					if (this.summary.month === summary.month) {
						this.summary.events = summary.events;
						this.summary.absences = summary.absences;
					}
				}
				if (this.currentMonthSummary) {
					this.currentMonthSummary.serverTime = summary.serverTime;
					this.currentMonthSummary.timeZone = summary.timeZone;
					this.currentMonthSummary.timeZoneAuthoritative = summary.timeZoneAuthoritative;
					this.currentMonthSummary.todayStatus = summary.todayStatus;
					this.currentMonthSummary.activeLeave = summary.activeLeave;
					if (this.currentMonthSummary.month === summary.month) {
						this.currentMonthSummary.events = summary.events;
						this.currentMonthSummary.absences = summary.absences;
					}
				}
			}
		});
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
		this.activeLoadCount += 1;
		this.isLoading = true;
		this.errorMessage = '';
		this.applyCachedSummary();
		const requestedMonth = this.selectedMonth;
		try {
			const selectedMonthRequest = this.serverClockSync.loadSummary(
				requestedMonth,
				(summary) => {
					this.summary = summary;
					this.selectedMonth = summary.month;
					this.isShowingCachedSummary = false;
					writeCachedAttendanceSummary(requestedMonth, summary);
				},
				selectedMonthSummaryLoadTarget
			);
			const currentMonthRequest = this.startCurrentMonthRequest();
			const next = await selectedMonthRequest;
			if (!next) return;
			if (currentMonthRequest) {
				await currentMonthRequest;
				return;
			}
			await this.refreshCurrentMonthSnapshot(next);
		} catch (error) {
			this.serverClockSync.invalidateLoadTarget(currentMonthSummaryLoadTarget);
			this.errorMessage = error instanceof Error ? error.message : this.loadFailedMessage;
			this.summary = null;
			this.currentMonthSummary = null;
			this.isShowingCachedSummary = false;
			clearCachedAttendanceSummaries();
		} finally {
			this.activeLoadCount -= 1;
			this.isLoading = this.activeLoadCount > 0;
		}
	}

	currentServerTime(monotonicTimestampMilliseconds: number = performance.now()): Date {
		if (!this.serverClock) return new Date(Number.NaN);
		return attendanceServerTime(this.serverClock, monotonicTimestampMilliseconds);
	}

	refreshServerClock(): Promise<boolean> {
		return this.serverClockSync.refresh(this.selectedMonth);
	}

	private applyCachedSummary() {
		if (this.summary) return;
		const cachedSummary = readCachedAttendanceSummary(this.selectedMonth);
		if (!cachedSummary) return;
		this.summary = cachedSummary;
		this.selectedMonth = cachedSummary.month;
		this.isShowingCachedSummary = true;
		const cachedCurrentMonth = readCachedAttendanceSummary(currentMonthInTimeZone(cachedSummary.timeZone));
		if (cachedCurrentMonth) this.currentMonthSummary = cachedCurrentMonth;
	}

	private startCurrentMonthRequest(): Promise<void> | undefined {
		const knownTimeZone = this.summary?.timeZone ?? '';
		if (!knownTimeZone) return undefined;
		const currentMonth = currentMonthInTimeZone(knownTimeZone);
		if (currentMonth === this.selectedMonth) return undefined;
		return this.serverClockSync
			.loadSummary(
				currentMonth,
				(summary) => {
					this.currentMonthSummary = summary;
					writeCachedAttendanceSummary(summary.month, summary);
				},
				currentMonthSummaryLoadTarget
			)
			.then(() => undefined)
			.catch(() => undefined);
	}

	private async refreshCurrentMonthSnapshot(filteredSummary: AttendanceSummary) {
		const currentMonth = currentMonthInTimeZone(filteredSummary.timeZone);
		if (filteredSummary.month === currentMonth) {
			this.serverClockSync.applySummaryToLoadTarget(
				filteredSummary,
				(summary) => {
					this.currentMonthSummary = summary;
				},
				currentMonthSummaryLoadTarget
			);
			return;
		}
		try {
			await this.serverClockSync.loadSummary(
				currentMonth,
				(summary) => {
					this.currentMonthSummary = summary;
					writeCachedAttendanceSummary(summary.month, summary);
				},
				currentMonthSummaryLoadTarget
			);
		} catch {
			this.currentMonthSummary = null;
		}
	}

	async toggleAttendance(
		kind?: AttendanceKind,
		locationID?: string,
		confirmEarlyReturn = false
	) {
		await toggleAttendanceOnServer(kind, locationID, confirmEarlyReturn);
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
