import { isPlainShortcut } from '$lib/keyboard-shortcut';
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

// The one answer to "am I working right now". The rail, the command palette and
// the attendance page all read it here, because a second copy of this drifts:
// leave outranks an open segment, and a reader that only looks at the segment
// offers to clock a person out of a day they are not working.
class AttendanceClock {
	summary = $state<AttendanceSummary | null>(null);
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

	// The attendance page has already fetched the month this is a day of, so it
	// hands it over rather than making the same request a second time.
	adopt = (summary: AttendanceSummary | null) => {
		if (summary) this.summary = summary;
	};

	load = async (): Promise<AttendanceSummary | null> => {
		try {
			this.summary = await fetchAttendanceSummary({
				month: currentMonthInTimeZone(this.summary?.timeZone)
			});
		} catch {
			this.summary = null;
		}
		return this.summary;
	};

	clock = async (kind: AttendanceKind, locationID: string, confirmEarlyReturn = false) => {
		if (this.isSubmitting) return;
		this.isSubmitting = true;
		try {
			await toggleAttendanceOnServer(kind, locationID, confirmEarlyReturn);
			await this.load();
		} finally {
			this.isSubmitting = false;
		}
	};

	handleShortcut = (event: KeyboardEvent) => {
		if (this.isMenuOpen || !isPlainShortcut(event, menuShortcutCode)) return;
		event.preventDefault();
		this.isMenuOpen = true;
	};
}

export const attendanceClock = new AttendanceClock();
