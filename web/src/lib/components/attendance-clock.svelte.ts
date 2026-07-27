import { isPlainShortcut } from '$lib/keyboard-shortcut';
import type { AttendanceSummary } from '../../routes/attendance/attendance-context.svelte';
import { computeDayEvents } from '../../routes/attendance/shared/attendance-aggregation';
import { todayDateInTimeZone } from '../../routes/attendance/shared/attendance-date';

export type AttendanceClockKind = 'clock_in' | 'clock_out';

const menuShortcutCode = 'Period';

class AttendanceClock {
	summary = $state<AttendanceSummary | null>(null);
	isMenuOpen = $state(false);
	isSubmitting = $state(false);

	locations = $derived(this.summary?.locations ?? []);

	activeSegment = $derived.by(() => {
		const summary = this.summary;
		if (!summary) return undefined;
		const today = todayDateInTimeZone(summary.timeZone);
		const myEvents = summary.events.filter((event) => event.email === summary.currentUserEmail);
		return computeDayEvents(today, myEvents, { currentDate: today }).activeSegment;
	});

	isClockedIn = $derived(this.activeSegment !== undefined);
	currentLocationID = $derived(this.activeSegment?.locationID ?? '');

	load = async (): Promise<AttendanceSummary | null> => {
		const response = await fetch('/attendance/api/summary', { credentials: 'include', cache: 'no-store' });
		if (!response.ok) return null;
		this.summary = (await response.json()) as AttendanceSummary;
		return this.summary;
	};

	clock = async (kind: AttendanceClockKind, locationID: string) => {
		if (this.isSubmitting) return;
		this.isSubmitting = true;
		try {
			const response = await fetch('/attendance/api/clock', {
				method: 'POST',
				credentials: 'include',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ kind, locationID })
			});
			if (!response.ok) return;
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
