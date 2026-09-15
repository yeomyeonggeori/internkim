import type { AttendanceWriteOutcome, AttendanceWriteResult } from '$lib/attendance/attendance-write';
import type { AddAttendanceEventRequest } from '../attendance-api';
import type { AttendanceKind, AttendanceSummary } from '../attendance-context.svelte';
import {
	fallbackFutureAttendanceLocalTime,
	timeInTimeZone,
	todayDateInTimeZone
} from '../shared/attendance-date';
import { attendanceSpanAdditionWriteOutcome } from './attendance-correction-access';

export type AttendanceRecordAdditionDependencies = Readonly<{
	getSummary: () => AttendanceSummary | null;
	getCurrentServerTime: () => Date;
	addEvent: (request: AddAttendanceEventRequest) => Promise<AttendanceWriteResult>;
	processingFailedMessage: string;
	partialSpanFailureMessage: string;
}>;

const defaultLocalTime = '09:00';

export function isAttendanceSpanInverted(startTime: string, endTime: string): boolean {
	return startTime !== '' && endTime !== '' && endTime < startTime;
}

export class AttendanceRecordAdditionState {
	isOpen = $state(false);
	email = $state('');
	localDate = $state('');
	startTime = $state('');
	endTime = $state('');
	locationID = $state('');
	reason = $state('');
	isSaving = $state(false);
	errorMessage = $state('');
	completedOutcome = $state<AttendanceWriteResult['outcome'] | null>(null);

	constructor(private readonly dependencies: AttendanceRecordAdditionDependencies) {}

	get outcome(): AttendanceWriteOutcome {
		const summary = this.dependencies.getSummary();
		if (!summary) return 'blocked';
		return attendanceSpanAdditionWriteOutcome(
			summary,
			{
				email: this.email,
				localDate: this.localDate,
				startTime: this.startTime,
				endTime: this.endTime
			},
			this.dependencies.getCurrentServerTime()
		);
	}

	get isSpanInverted(): boolean {
		return isAttendanceSpanInverted(this.startTime, this.endTime);
	}

	get isEndTimeMissing(): boolean {
		const summary = this.dependencies.getSummary();
		const currentTime = this.dependencies.getCurrentServerTime();
		if (!summary || !Number.isFinite(currentTime.getTime()) || this.localDate === '') return false;
		return (
			this.endTime === '' &&
			this.localDate !== todayDateInTimeZone(summary.timeZone, currentTime)
		);
	}

	get canSubmit(): boolean {
		return (
			!this.isSaving &&
			this.email !== '' &&
			this.localDate !== '' &&
			(this.startTime !== '' || this.endTime !== '') &&
			!this.isSpanInverted &&
			!this.isEndTimeMissing &&
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
		this.localDate = localDate;
		this.startTime = openingLocalTime(summary, localDate, currentTime);
		this.endTime = '';
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

	updateStartTime(localTime: string): string {
		this.startTime = this.boundedLocalTime(localTime);
		return this.startTime;
	}

	updateEndTime(localTime: string): string {
		this.endTime = this.boundedLocalTime(localTime);
		return this.endTime;
	}

	async submit(): Promise<void> {
		if (!this.canSubmit) return;
		this.isSaving = true;
		this.errorMessage = '';
		const reason = this.reason.trim();
		let startWasWritten = false;
		try {
			const outcomes: AttendanceWriteResult['outcome'][] = [];
			if (this.startTime !== '') {
				outcomes.push((await this.writeEvent('clock_in', this.startTime, reason)).outcome);
				startWasWritten = true;
			}
			if (this.endTime !== '') {
				outcomes.push((await this.writeEvent('clock_out', this.endTime, reason)).outcome);
			}
			this.isOpen = false;
			this.completedOutcome = combinedOutcome(outcomes);
		} catch (failure) {
			const message =
				failure instanceof Error ? failure.message : this.dependencies.processingFailedMessage;
			this.errorMessage = startWasWritten
				? `${this.dependencies.partialSpanFailureMessage} ${message}`
				: message;
		} finally {
			this.isSaving = false;
		}
	}

	private boundedLocalTime(localTime: string): string {
		const summary = this.dependencies.getSummary();
		return fallbackFutureAttendanceLocalTime(
			this.localDate,
			localTime,
			summary?.timeZone,
			this.dependencies.getCurrentServerTime()
		);
	}

	private writeEvent(
		kind: AttendanceKind,
		localTime: string,
		reason: string
	): Promise<AttendanceWriteResult> {
		return this.dependencies.addEvent({
			email: this.email,
			kind,
			localDate: this.localDate,
			localTime,
			locationID: this.locationID,
			reason
		});
	}
}

function openingLocalTime(summary: AttendanceSummary, localDate: string, currentTime: Date): string {
	if (!Number.isFinite(currentTime.getTime())) return defaultLocalTime;
	if (localDate !== todayDateInTimeZone(summary.timeZone, currentTime)) return defaultLocalTime;
	return timeInTimeZone(summary.timeZone, currentTime);
}

function combinedOutcome(
	outcomes: AttendanceWriteResult['outcome'][]
): AttendanceWriteResult['outcome'] {
	return outcomes.includes('asked') ? 'asked' : 'saved';
}
