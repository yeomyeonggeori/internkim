import { toast } from 'svelte-sonner';
import { isPlainShortcut } from '$lib/keyboard-shortcut';
import { feelHaptic } from '$lib/native-shell/haptics';
import { lockScreenRefusesTheClock } from '$lib/widget/attendance-lock-screen';
import { createPageText } from '$lib/i18n/page-text.svelte';
import { attendanceText } from '../../routes/attendance/text';
import { addAttendanceEvent, toggleAttendanceOnServer } from '../../routes/attendance/attendance-api';
import { supabaseCurrentAttendance } from './supabase-current-attendance';
import { attendanceEventFromClock } from './supabase-attendance';
import type { AttendanceWriteEvent } from './attendance-write';
import type {
	AttendanceKind,
	AttendanceSummary
} from '../../routes/attendance/attendance-context.svelte';
import { computeDayEvents, statusForDay } from '../../routes/attendance/shared/attendance-aggregation';
import { clockInNobodyClosed } from '../../routes/attendance/shared/attendance-work-segments';
import {
	todayDateInTimeZone
} from '../../routes/attendance/shared/attendance-date';

const menuShortcutCode = 'Period';
const ownStateFreshForMilliseconds = 15000;
const text = createPageText(attendanceText);

class MyAttendanceToday {
	summary = $state<AttendanceSummary | null>(null);
	loadFailure = $state<string>('');
	clockFailure = $state<string>('');
	isMenuOpen = $state(false);
	isSubmitting = $state(false);
	private clockEventHandler: ((event: AttendanceWriteEvent) => void) | undefined;
	private clockMutationHandler: (() => void) | undefined;
	private clockMutationSequence = 0;
	private authorityGeneration = 0;
	private loadPromise: Promise<AttendanceSummary | null> | undefined;
	private loadedAt = $state(Number.NEGATIVE_INFINITY);
	private hasFreshRead = false;

	private today = $derived(todayDateInTimeZone(this.summary?.timeZone, this.currentServerTime()));
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

	setClockEventHandler = (handler: ((event: AttendanceWriteEvent) => void) | undefined) => {
		this.clockEventHandler = handler;
	};

	setClockMutationHandler = (handler: (() => void) | undefined) => {
		this.clockMutationHandler = handler;
	};

	clear = (): void => {
		this.authorityGeneration += 1;
		this.invalidateRead();
		this.summary = null;
		this.loadFailure = '';
		this.clockFailure = '';
		this.isSubmitting = false;
		this.loadedAt = Number.NEGATIVE_INFINITY;
	};

	refresh = (): Promise<AttendanceSummary | null> => {
		this.invalidateRead();
		return this.load(true);
	};

	private invalidateRead(): void {
		this.clockMutationSequence += 1;
		this.loadPromise = undefined;
		this.hasFreshRead = false;
	}

	load = (force = false): Promise<AttendanceSummary | null> => {
		if (this.loadPromise) return this.loadPromise;
		if (!force && this.isFresh()) return Promise.resolve(this.summary);
		const loadPromise = this.loadFromServer();
		const trackedLoad = loadPromise.finally(() => {
			if (this.loadPromise === trackedLoad) this.loadPromise = undefined;
		});
		this.loadPromise = trackedLoad;
		return trackedLoad;
	};

	private currentServerTime(): Date {
		const serverTime = Date.parse(this.summary?.serverTime ?? '');
		if (!Number.isFinite(serverTime)) return new Date();
		return new Date(serverTime + performance.now() - this.loadedAt);
	}

	private isFresh(): boolean {
		const summary = this.summary;
		if (!this.hasFreshRead || !summary?.serverTime || performance.now() - this.loadedAt >= ownStateFreshForMilliseconds) return false;
		return todayDateInTimeZone(summary.timeZone, this.currentServerTime()) === todayDateInTimeZone(summary.timeZone, new Date(summary.serverTime));
	}

	private loadFromServer = async (): Promise<AttendanceSummary | null> => {
		const loadSequence = this.clockMutationSequence;
		try {
			const summary = await supabaseCurrentAttendance();
			if (loadSequence !== this.clockMutationSequence) return this.summary;
			this.summary = summary;
			this.loadedAt = performance.now();
			this.hasFreshRead = true;
			this.loadFailure = '';
		} catch (failure) {
			if (loadSequence !== this.clockMutationSequence) return this.summary;
			this.summary = null;
			this.hasFreshRead = false;
			this.loadFailure = failure instanceof Error ? failure.message : String(failure);
		}
		return this.summary;
	};

	clock = async (kind: AttendanceKind, locationID: string, confirmedEarlyReturn = false) => {
		if (this.isSubmitting) return;
		const authorityGeneration = this.authorityGeneration;
		this.isSubmitting = true;
		this.clockFailure = '';
		try {
			const result = await toggleAttendanceOnServer(kind, locationID, confirmedEarlyReturn);
			if (authorityGeneration !== this.authorityGeneration) return;
			this.invalidateRead();
			if (result?.event && this.applyClockEvent(result.event)) {
				this.clockEventHandler?.(result.event);
			} else {
				await this.load(true);
			}
			if (authorityGeneration !== this.authorityGeneration) return;
			this.clockMutationHandler?.();
			feelHaptic('success');
			toast.success(result?.removed ? this.takenBackClockMessage(kind) : this.recordedClockMessage(kind));
			if (kind === 'clock_in' && (await lockScreenRefusesTheClock())) toast.info(text.lockScreenOff);
		} catch (failure) {
			if (authorityGeneration !== this.authorityGeneration) return;
			this.clockFailure = failure instanceof Error ? failure.message : String(failure);
			toast.error(text.clockFailed, { description: this.clockFailure });
			throw failure;
		} finally {
			if (authorityGeneration === this.authorityGeneration) this.isSubmitting = false;
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
		const authorityGeneration = this.authorityGeneration;
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
			if (authorityGeneration !== this.authorityGeneration) return;
			this.clockMutationHandler?.();
			await this.refresh();
			if (authorityGeneration !== this.authorityGeneration) return;
			await toggleAttendanceOnServer('clock_in', locationID, false);
			if (authorityGeneration !== this.authorityGeneration) return;
			this.clockMutationHandler?.();
			await this.refresh();
			if (authorityGeneration !== this.authorityGeneration) return;
			feelHaptic('success');
			toast.success(this.recordedClockMessage('clock_in'));
		} catch (failure) {
			if (authorityGeneration !== this.authorityGeneration) return;
			this.clockFailure = failure instanceof Error ? failure.message : String(failure);
			toast.error(text.clockFailed, { description: this.clockFailure });
			throw failure;
		} finally {
			if (authorityGeneration === this.authorityGeneration) this.isSubmitting = false;
		}
	};

	private takenBackClockMessage = (kind: AttendanceKind): string =>
		kind === 'clock_out' ? text.clockInTakenBack : text.clockOutTakenBack;

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
