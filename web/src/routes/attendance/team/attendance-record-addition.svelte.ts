import type { AttendanceWriteOutcome, AttendanceWriteResult } from '$lib/attendance/attendance-write';
import type { AddAttendanceEventRequest } from '../attendance-api';
import type { AttendanceKind, AttendanceSummary } from '../attendance-context.svelte';
import {
	fallbackFutureAttendanceLocalTime,
	timeInTimeZone,
	todayDateInTimeZone
} from '../shared/attendance-date';
import { attendanceAdditionWriteOutcome } from './attendance-correction-access';

export type AttendanceRecordAdditionDependencies = Readonly<{
	getSummary: () => AttendanceSummary | null;
	getCurrentServerTime: () => Date;
	addEvent: (request: AddAttendanceEventRequest) => Promise<AttendanceWriteResult>;
	processingFailedMessage: string;
}>;

const defaultLocalTime = '09:00';

export class AttendanceRecordAdditionState {
	isOpen = $state(false);
	email = $state('');
	kind = $state<AttendanceKind>('clock_in');
	localDate = $state('');
	localTime = $state('');
	locationID = $state('');
	reason = $state('');
	isSaving = $state(false);
	errorMessage = $state('');
	completedOutcome = $state<AttendanceWriteResult['outcome'] | null>(null);

	constructor(private readonly dependencies: AttendanceRecordAdditionDependencies) {}

	get outcome(): AttendanceWriteOutcome {
		const summary = this.dependencies.getSummary();
		if (!summary) return 'blocked';
		return attendanceAdditionWriteOutcome(
			summary,
			{ email: this.email, localDate: this.localDate, localTime: this.localTime },
			this.dependencies.getCurrentServerTime()
		);
	}

	get canSubmit(): boolean {
		return (
			!this.isSaving &&
			this.email !== '' &&
			this.localDate !== '' &&
			this.localTime !== '' &&
			this.reason.trim() !== '' &&
			this.outcome !== 'blocked'
		);
	}

	get maximumTime(): string | undefined {
		const summary = this.dependencies.getSummary();
		const currentTime = this.dependencies.getCurrentServerTime();
		if (!summary || !Number.isFinite(currentTime.getTime())) return undefined;
		if (this.localDate !== todayDateInTimeZone(summary.timeZone, currentTime)) return undefined;
		return timeInTimeZone(summary.timeZone, currentTime);
	}

	open(localDate: string, email: string): void {
		const summary = this.dependencies.getSummary();
		if (!summary) return;
		const currentTime = this.dependencies.getCurrentServerTime();
		this.email = email;
		this.kind = 'clock_in';
		this.localDate = localDate;
		this.localTime = openingLocalTime(summary, localDate, currentTime);
		this.locationID = summary.locations[0]?.id ?? '';
		this.reason = '';
		this.errorMessage = '';
		this.completedOutcome = null;
		this.isOpen = true;
	}

	close(): void {
		if (this.isSaving) return;
		this.isOpen = false;
	}

	updateLocalTime(localTime: string): string {
		const summary = this.dependencies.getSummary();
		this.localTime = fallbackFutureAttendanceLocalTime(
			this.localDate,
			localTime,
			summary?.timeZone,
			this.dependencies.getCurrentServerTime()
		);
		return this.localTime;
	}

	async submit(): Promise<void> {
		if (!this.canSubmit) return;
		this.isSaving = true;
		this.errorMessage = '';
		try {
			const result = await this.dependencies.addEvent({
				email: this.email,
				kind: this.kind,
				localDate: this.localDate,
				localTime: this.localTime,
				locationID: this.locationID,
				reason: this.reason.trim()
			});
			this.isOpen = false;
			this.completedOutcome = result.outcome;
		} catch (failure) {
			this.errorMessage =
				failure instanceof Error ? failure.message : this.dependencies.processingFailedMessage;
		} finally {
			this.isSaving = false;
		}
	}
}

function openingLocalTime(summary: AttendanceSummary, localDate: string, currentTime: Date): string {
	if (!Number.isFinite(currentTime.getTime())) return defaultLocalTime;
	if (localDate !== todayDateInTimeZone(summary.timeZone, currentTime)) return defaultLocalTime;
	return timeInTimeZone(summary.timeZone, currentTime);
}
