import type { AttendanceWriteOutcome, AttendanceWriteResult } from '$lib/attendance/attendance-write';
import type { AttendanceText } from '../text';

export type AttendanceRecordText = AttendanceText['records'];

export function attendanceWriteIntent(
	outcome: AttendanceWriteOutcome,
	text: AttendanceRecordText
): string {
	if (outcome === 'blocked') return text.blocked;
	if (outcome === 'asked') return text.asksAnAdministrator;
	return text.savesImmediately;
}

export function attendanceWriteConfirmation(
	outcome: AttendanceWriteResult['outcome'],
	text: AttendanceRecordText
): string {
	return outcome === 'asked' ? text.askedAnAdministrator : text.saved;
}

export function attendanceWriteSubmitLabel(
	outcome: AttendanceWriteOutcome,
	text: AttendanceRecordText,
	immediateLabel: string
): string {
	return outcome === 'asked' ? text.askSubmit : immediateLabel;
}
