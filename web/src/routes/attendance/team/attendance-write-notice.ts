import type { AttendanceWriteOutcome, AttendanceWriteResult } from '$lib/attendance/attendance-write';
import type { AttendanceText } from '../text';

export type AttendanceRecordText = AttendanceText['records'];

export function attendanceWriteIntent(
	outcome: AttendanceWriteOutcome,
	text: AttendanceRecordText
): string {
	if (outcome === 'blocked') return text.blocked;
	if (outcome === 'requested') return text.requestsApproval;
	return text.savesImmediately;
}

export function attendanceWriteConfirmation(
	outcome: AttendanceWriteResult['outcome'],
	text: AttendanceRecordText
): string {
	return outcome === 'requested' ? text.requested : text.saved;
}

export function attendanceWriteSubmitLabel(
	outcome: AttendanceWriteOutcome,
	text: AttendanceRecordText,
	immediateLabel: string
): string {
	return outcome === 'requested' ? text.requestSubmit : immediateLabel;
}
