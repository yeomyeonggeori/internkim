import { toast } from 'svelte-sonner';
import { isPlainShortcut } from '$lib/keyboard-shortcut';
import { createPageText } from '$lib/i18n/page-text.svelte';
import { attendanceText } from '../../routes/attendance/text';
import { fetchAttendanceSummary, toggleAttendanceOnServer } from '../../routes/attendance/attendance-api';
import type {
	AttendanceKind,
	AttendanceSummary
} from '../../routes/attendance/attendance-context.svelte';
import { computeDayEvents, statusForDay } from '../../routes/attendance/shared/attendance-aggregation';
import {
	currentMonthInTimeZone,
	todayDateInTimeZone
} from '../../routes/attendance/shared/attendance-date';

const menuShortcutCode = 'Period';
const text = createPageText(attendanceText);

class MyAttendanceToday {
	summary = $state<AttendanceSummary | null>(null);
	loadFailure = $state<string>('');
	clockFailure = $state<string>('');
	isMenuOpen = $state(false);
	isSubmitting = $state(false);

	private today = $derived(todayDateInTimeZone(this.summary?.timeZone));
	private myEvents = $derived(
		(this.summary?.events ?? []).filter((event) => event.email === this.summary?.currentUserEmail)
	);
	private myAbsences = $derived(
		(this.summary?.absences ?? []).filter(
			(absence) => absence.email === this.summary?.currentUserEmail
		)
	);

	day = $derived(computeDayEvents(this.today, this.myEvents, { currentDate: this.today }));
	status = $derived(statusForDay(this.today, this.myEvents, this.myAbsences, this.today));
	activeLeave = $derived(this.summary?.activeLeave);
	locations = $derived(this.summary?.locations ?? []);
	defaultLocation = $derived(this.locations.find((location) => location.isDefault) ?? this.locations[0]);
	activeSegment = $derived(this.day.activeSegment);
	currentLocationID = $derived(this.activeSegment?.locationID ?? '');
	nextKind = $derived<AttendanceKind>(
		this.activeLeave ? 'clock_in' : this.status === 'working' ? 'clock_out' : 'clock_in'
	);

	adoptSummary = (summary: AttendanceSummary | null) => {
		if (summary) this.summary = summary;
	};

	load = async (): Promise<AttendanceSummary | null> => {
		try {
			this.summary = await fetchAttendanceSummary({
				month: currentMonthInTimeZone(this.summary?.timeZone)
			});
			this.loadFailure = '';
		} catch (failure) {
			this.summary = null;
			this.loadFailure = failure instanceof Error ? failure.message : String(failure);
		}
		return this.summary;
	};

	clock = async (kind: AttendanceKind, locationID: string) => {
		if (this.isSubmitting) return;
		this.isSubmitting = true;
		this.clockFailure = '';
		try {
			await toggleAttendanceOnServer(kind, locationID);
			await this.load();
			toast.success(this.recordedClockMessage(kind));
		} catch (failure) {
			this.clockFailure = failure instanceof Error ? failure.message : String(failure);
			toast.error(text.clockFailed, { description: this.clockFailure });
			throw failure;
		} finally {
			this.isSubmitting = false;
		}
	};

	private recordedClockMessage = (kind: AttendanceKind): string => {
		if (kind === 'clock_out') return text.clockedOut;
		const locationName = this.locations.find(
			(location) => location.id === this.currentLocationID
		)?.name;
		if (!locationName) return text.clockedIn;
		return text.clockedInAtTemplate.replace('{location}', locationName);
	};

	handleShortcut = (event: KeyboardEvent) => {
		if (this.isMenuOpen || !isPlainShortcut(event, menuShortcutCode)) return;
		event.preventDefault();
		this.isMenuOpen = true;
	};
}

export const myAttendanceToday = new MyAttendanceToday();
