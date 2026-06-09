import { getContext, setContext } from 'svelte';
import {
	createAttendanceAbsence,
	type CreateAttendanceAbsenceRequest,
	fetchAttendanceSummary,
	toggleAttendanceOnServer,
	updateAttendanceTeamViewVisibility
} from './attendance-api';
import { readPersistedAttendanceFilters, writePersistedAttendanceFilters } from './attendance-storage';
import { currentMonthInTimeZone } from './shared/attendance-date';

export type AttendanceKind = 'clock_in' | 'clock_out';
export type AttendanceAbsenceKind = 'leave' | 'business_trip' | 'day_off' | 'other';
export type ChartMode = 'day' | 'week' | 'month';
export type AttendanceTab = 'team' | 'personal';
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
	manualEntry?: boolean;
};

export type AttendanceAbsence = {
	id: string;
	email: string;
	kind: AttendanceAbsenceKind;
	labelKey: AttendanceAbsenceKind;
	date: string;
	reason?: string;
	createdBy?: string;
	createdAt: string;
	canceledAt?: string;
};

export type AttendanceSummary = {
	month: string;
	currentUserEmail: string;
	isAdmin: boolean;
	timeZone: string;
	events: AttendanceEvent[];
	absences: AttendanceAbsence[];
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
	tab = $state<AttendanceTab>('team');
	selectedDate = $state<string>('');
	isLoading = $state<boolean>(false);
	errorMessage = $state<string>('');

	private loadFailedMessage: string;
	private tabExplicitlySet = false;

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

	setTab(value: AttendanceTab) {
		this.tab = value;
		this.tabExplicitlySet = true;
	}

	async load() {
		this.isLoading = true;
		this.errorMessage = '';
		try {
			const next = await this.fetchSummaryForMonth(this.selectedMonth);
			this.summary = next;
			this.selectedMonth = next.month;
			const personalOnly = !next.isAdmin && next.teamViewBlocked;
			if (!this.tabExplicitlySet) {
				this.tab = next.isAdmin ? 'team' : 'personal';
			}
			if (personalOnly && this.tab === 'team') {
				this.tab = 'personal';
			}
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

	overrideLocation(eventID: string, newLocationID: string) {
		if (!this.summary) return;
		const overriddenAt = new Date().toISOString();
		const overriddenBy = this.summary.currentUserEmail;
		this.applyOverride(this.summary, eventID, newLocationID, overriddenAt, overriddenBy);
		if (this.currentMonthSummary && this.currentMonthSummary !== this.summary) {
			this.applyOverride(this.currentMonthSummary, eventID, newLocationID, overriddenAt, overriddenBy);
		}
	}

	private applyOverride(target: AttendanceSummary, eventID: string, newLocationID: string, overriddenAt: string, overriddenBy: string) {
		const location = target.locations.find((l) => l.id === newLocationID);
		target.events = target.events.map((event) =>
			event.id === eventID
				? {
						...event,
						parsedAs: event.parsedAs ?? { kind: event.kind, locationID: event.locationID },
						locationID: newLocationID,
						locationName: location?.name ?? event.locationName,
						overriddenBy,
						overriddenAt,
					}
				: event
		);
	}

	dismissEvent(eventID: string, reason: string) {
		if (!this.summary) return;
		const canceledAt = new Date().toISOString();
		this.applyDismiss(this.summary, eventID, canceledAt, reason);
		if (this.currentMonthSummary && this.currentMonthSummary !== this.summary) {
			this.applyDismiss(this.currentMonthSummary, eventID, canceledAt, reason);
		}
	}

	private applyDismiss(target: AttendanceSummary, eventID: string, canceledAt: string, reason: string) {
		target.events = target.events.map((event) =>
			event.id === eventID
				? { ...event, canceledAt, cancelReason: reason }
				: event
		);
	}

	confirmClassification(eventID: string) {
		if (!this.summary) return;
		const overriddenAt = new Date().toISOString();
		const overriddenBy = this.summary.currentUserEmail;
		this.applyConfirm(this.summary, eventID, overriddenAt, overriddenBy);
		if (this.currentMonthSummary && this.currentMonthSummary !== this.summary) {
			this.applyConfirm(this.currentMonthSummary, eventID, overriddenAt, overriddenBy);
		}
	}

	private applyConfirm(target: AttendanceSummary, eventID: string, overriddenAt: string, overriddenBy: string) {
		target.events = target.events.map((event) =>
			event.id === eventID
				? {
						...event,
						parsedAs: event.parsedAs ?? { kind: event.kind, locationID: event.locationID },
						overriddenBy,
						overriddenAt,
					}
				: event
		);
	}

	async updateTeamViewVisibility(visible: boolean) {
		await updateAttendanceTeamViewVisibility(visible);
		await this.load();
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
}

const KEY = Symbol('attendance-state');
export const setAttendanceState = (s: AttendanceState) => setContext(KEY, s);
export const getAttendanceState = (): AttendanceState => {
	const s = getContext<AttendanceState | undefined>(KEY);
	if (!s) throw new Error('AttendanceState not provided — must wrap in attendance/+layout.svelte');
	return s;
};
