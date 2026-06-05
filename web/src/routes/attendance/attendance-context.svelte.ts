import { getContext, setContext } from 'svelte';
import {
	fetchAttendanceSummary,
	toggleAttendanceOnServer,
	updateAttendanceEventLocation,
	updateAttendanceTeamViewVisibility
} from './attendance-api';
import { readPersistedAttendanceFilters, writePersistedAttendanceFilters } from './attendance-storage';
import { currentMonthInTimeZone } from './shared/attendance-date';

export type AttendanceKind = 'clock_in' | 'clock_out';
export type ChartMode = 'day' | 'week' | 'month';
export type AttendanceTab = 'team' | 'personal';
export type AttendancePresence = 'online' | 'away' | 'offline' | 'dnd';
export type AttendanceSource = 'mattermost_button' | 'mattermost_post';

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
	source: AttendanceSource | string;
	resultPostID: string;
	locationID?: string;
	locationName?: string;
	canceledAt?: string;
	cancelReason?: string;
	sourceMessage?: string;
	confidence?: number;
	parsedAs?: { kind?: AttendanceKind; locationID?: string };
	originalLocationID?: string;
	originalLocationName?: string;
	overriddenBy?: string;
	overriddenAt?: string;
	manualEntry?: boolean;
};

export type AttendanceSummary = {
	month: string;
	currentUserEmail: string;
	isAdmin: boolean;
	timeZone: string;
	events: AttendanceEvent[];
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
	selectedEmail = $state<string>('');
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
		if (persisted.selectedEmail) this.selectedEmail = persisted.selectedEmail;
		if (persisted.chartMode) this.chartMode = persisted.chartMode;
	}

	persistFilters() {
		writePersistedAttendanceFilters({
			selectedMonth: this.selectedMonth,
			selectedEmail: this.selectedEmail,
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
			if (personalOnly && this.selectedEmail) {
				this.selectedEmail = '';
				this.persistFilters();
			}
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
		return fetchAttendanceSummary({ month, selectedEmail: this.selectedEmail });
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

	async overrideLocation(eventID: string, newLocationID: string) {
		const updatedEvent = await updateAttendanceEventLocation(eventID, newLocationID);
		if (this.summary) {
			this.replaceEvent(this.summary, updatedEvent);
		}
		if (this.currentMonthSummary && this.currentMonthSummary !== this.summary) {
			this.replaceEvent(this.currentMonthSummary, updatedEvent);
		}
	}

	private replaceEvent(target: AttendanceSummary, updatedEvent: AttendanceEvent) {
		target.events = target.events.map((event) =>
			event.id === updatedEvent.id
				? {
						...event,
						...updatedEvent
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
}

const KEY = Symbol('attendance-state');
export const setAttendanceState = (s: AttendanceState) => setContext(KEY, s);
export const getAttendanceState = (): AttendanceState => {
	const s = getContext<AttendanceState | undefined>(KEY);
	if (!s) throw new Error('AttendanceState not provided — must wrap in attendance/+layout.svelte');
	return s;
};
