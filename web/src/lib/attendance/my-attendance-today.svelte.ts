import { toast } from 'svelte-sonner';
import { isPlainShortcut } from '$lib/keyboard-shortcut';
import { lockScreenRefusesTheClock } from '$lib/widget/attendance-lock-screen';
import { createPageText } from '$lib/i18n/page-text.svelte';
import { attendanceText } from '../../routes/attendance/text';
import { addAttendanceEvent, fetchAttendanceSummary, toggleAttendanceOnServer } from '../../routes/attendance/attendance-api';
import { attendanceEventFromClock } from './supabase-attendance';
import type { AttendanceWriteEvent } from './attendance-write';
import type {
	AttendanceKind,
	AttendanceSummary
} from '../../routes/attendance/attendance-context.svelte';
import { computeDayEvents, statusForDay } from '../../routes/attendance/shared/attendance-aggregation';
import { clockInNobodyClosed } from '../../routes/attendance/shared/attendance-work-segments';
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
	private clockEventHandler: ((event: AttendanceWriteEvent) => void) | undefined;
	private clockMutationSequence = 0;
	private loadPromise: Promise<AttendanceSummary | null> | undefined;

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
	// The record refuses a clock-in while an earlier one is still open, and that
	// one is invisible here once its day has passed. Clocking in has to close it
	// first, so the screen has to know it is there.
	clockInNobodyClosed = $derived(
		this.nextKind === 'clock_in' ? clockInNobodyClosed(this.myEvents) : undefined
	);

	adoptSummary = (summary: AttendanceSummary | null) => {
		if (summary) this.summary = summary;
	};

	setClockEventHandler = (handler: ((event: AttendanceWriteEvent) => void) | undefined) => {
		this.clockEventHandler = handler;
	};

	clear = (): void => {
		this.clockMutationSequence += 1;
		this.summary = null;
		this.loadFailure = '';
		this.clockFailure = '';
		this.loadPromise = undefined;
	};

	load = (): Promise<AttendanceSummary | null> => {
		if (this.loadPromise) return this.loadPromise;
		const loadPromise = this.loadFromServer();
		const trackedLoad = loadPromise.finally(() => {
			if (this.loadPromise === trackedLoad) this.loadPromise = undefined;
		});
		this.loadPromise = trackedLoad;
		return trackedLoad;
	};

	private loadFromServer = async (): Promise<AttendanceSummary | null> => {
		const loadSequence = this.clockMutationSequence;
		try {
			const summary = await fetchAttendanceSummary({
				month: currentMonthInTimeZone(this.summary?.timeZone)
			});
			if (loadSequence !== this.clockMutationSequence) return this.summary;
			this.summary = summary;
			this.loadFailure = '';
		} catch (failure) {
			if (loadSequence !== this.clockMutationSequence) return this.summary;
			this.summary = null;
			this.loadFailure = failure instanceof Error ? failure.message : String(failure);
		}
		return this.summary;
	};

	clock = async (kind: AttendanceKind, locationID: string, confirmedEarlyReturn = false) => {
		if (this.isSubmitting) return;
		this.isSubmitting = true;
		this.clockFailure = '';
		try {
			const result = await toggleAttendanceOnServer(kind, locationID, confirmedEarlyReturn);
			this.clockMutationSequence += 1;
			this.loadPromise = undefined;
			if (result?.event && this.applyClockEvent(result.event)) {
				this.clockEventHandler?.(result.event);
			} else {
				await this.load();
			}
			toast.success(this.recordedClockMessage(kind));
			if (kind === 'clock_in' && (await lockScreenRefusesTheClock())) toast.info(text.lockScreenOff);
		} catch (failure) {
			this.clockFailure = failure instanceof Error ? failure.message : String(failure);
			toast.error(text.clockFailed, { description: this.clockFailure });
			throw failure;
		} finally {
			this.isSubmitting = false;
		}
	};

	private applyClockEvent = (event: AttendanceWriteEvent): boolean => {
		const summary = this.summary;
		if (!summary || event.personID !== summary.currentMemberID) return false;
		if (summary.events.some((candidate) => candidate.id === event.id)) return true;
		const attendanceEvent = attendanceEventFromClock(event, summary.currentUserEmail, summary.timeZone);
		summary.events = [...summary.events, attendanceEvent].sort((left, right) =>
			left.occurredAt.localeCompare(right.occurredAt)
		);
		if (attendanceEvent.localDate === todayDateInTimeZone(summary.timeZone)) {
			summary.todayStatus = event.kind === 'clock_in' ? 'working' : 'done';
		}
		return true;
	};

	// Closing the older day and clocking in are one act to the person doing it,
	// so a clock-in that never got its clock-out cannot leave them recorded as
	// still at work yesterday. The close goes first: the record refuses today's
	// clock-in until it lands.
	closeAndClockIn = async (clockOutTime: string, locationID: string) => {
		const stillOpen = this.clockInNobodyClosed;
		if (!stillOpen || this.isSubmitting) return;
		this.isSubmitting = true;
		this.clockFailure = '';
		try {
			await addAttendanceEvent({
				email: this.summary?.currentUserEmail ?? '',
				kind: 'clock_out',
				localDate: stillOpen.localDate,
				localTime: clockOutTime,
				locationID: '',
				reason: text.clockOutNobodyRecordedReason
			});
			await toggleAttendanceOnServer('clock_in', locationID, false);
			this.clockMutationSequence += 1;
			this.loadPromise = undefined;
			await this.load();
			toast.success(this.recordedClockMessage('clock_in'));
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
