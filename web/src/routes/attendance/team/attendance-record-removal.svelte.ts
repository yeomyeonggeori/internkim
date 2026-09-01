import type { AttendanceWriteOutcome, AttendanceWriteResult } from '$lib/attendance/attendance-write';
import type { RemoveAttendanceEventRequest } from '../attendance-api';
import type { AttendanceEvent, AttendanceSummary } from '../attendance-context.svelte';
import { attendanceEventWriteOutcome } from './attendance-correction-access';

export type AttendanceRecordRemovalDependencies = Readonly<{
	getSummary: () => AttendanceSummary | null;
	getCurrentServerTime: () => Date;
	removeEvent: (request: RemoveAttendanceEventRequest) => Promise<AttendanceWriteResult>;
	processingFailedMessage: string;
}>;

export class AttendanceRecordRemovalState {
	isOpen = $state(false);
	eventID = $state('');
	reason = $state('');
	isSaving = $state(false);
	errorMessage = $state('');
	completedOutcome = $state<AttendanceWriteResult['outcome'] | null>(null);

	constructor(private readonly dependencies: AttendanceRecordRemovalDependencies) {}

	get event(): AttendanceEvent | undefined {
		return this.dependencies.getSummary()?.events.find((event) => event.id === this.eventID);
	}

	get outcome(): AttendanceWriteOutcome {
		const summary = this.dependencies.getSummary();
		const event = this.event;
		if (!summary || !event) return 'blocked';
		return attendanceEventWriteOutcome(summary, event, this.dependencies.getCurrentServerTime());
	}

	get canSubmit(): boolean {
		return !this.isSaving && this.reason.trim() !== '' && this.outcome !== 'blocked';
	}

	open(eventID: string): void {
		this.eventID = eventID;
		this.reason = '';
		this.errorMessage = '';
		this.completedOutcome = null;
		this.isOpen = true;
	}

	close(): void {
		if (this.isSaving) return;
		this.isOpen = false;
	}

	async submit(): Promise<void> {
		if (!this.canSubmit) return;
		this.isSaving = true;
		this.errorMessage = '';
		try {
			const result = await this.dependencies.removeEvent({
				eventID: this.eventID,
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
