import { isPlainShortcut } from '$lib/keyboard-shortcut';
import { fetchAttendanceSummary, toggleAttendanceOnServer } from '../../routes/attendance/attendance-api';
import type { AttendanceSummary } from '../../routes/attendance/attendance-context.svelte';
import { computeDayEvents } from '../../routes/attendance/shared/attendance-aggregation';
import { currentMonthInTimeZone, todayDateInTimeZone } from '../../routes/attendance/shared/attendance-date';

export type AttendanceClockKind = 'clock_in' | 'clock_out';

const menuShortcutCode = 'Period';

class AttendanceClock {
	summary = $state<AttendanceSummary | null>(null);
	isMenuOpen = $state(false);
	isSubmitting = $state(false);

	locations = $derived(this.summary?.locations ?? []);
	defaultLocation = $derived(this.locations.find((location) => location.isDefault) ?? this.locations[0]);

	activeSegment = $derived.by(() => {
		const summary = this.summary;
		if (!summary) return undefined;
		const today = todayDateInTimeZone(summary.timeZone);
		const myEvents = summary.events.filter((event) => event.email === summary.currentUserEmail);
		return computeDayEvents(today, myEvents, { currentDate: today }).activeSegment;
	});

	isClockedIn = $derived(this.activeSegment !== undefined);
	currentLocationID = $derived(this.activeSegment?.locationID ?? '');

	// A device answers /attendance/api/*; a company on the central plane reads the
	// tables itself, and attendance-api is where that fork lives.
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

	clock = async (kind: AttendanceClockKind, locationID: string) => {
		if (this.isSubmitting) return;
		this.isSubmitting = true;
		try {
			await toggleAttendanceOnServer(kind, locationID);
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
