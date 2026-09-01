import type { AttendanceKind } from '../attendance-context.svelte';
import type { AttendanceApprovalDetail } from './attendance-approval-types';

export type AttendanceApprovalLabels = {
	attendanceAdd: string;
	attendanceEdit: string;
	attendanceRemove: string;
	clockIn: string;
	clockOut: string;
};

export type AttendanceApprovalKnownEvent = {
	localDate: string;
	localTime: string;
	kind: AttendanceKind;
};

export type AttendanceApprovalEventLookup = (
	eventID: string
) => AttendanceApprovalKnownEvent | undefined;

export function attendanceApprovalKindLabel(
	detail: AttendanceApprovalDetail,
	labels: AttendanceApprovalLabels
): string {
	if (detail.kind === 'attendance_add') return labels.attendanceAdd;
	if (detail.kind === 'attendance_remove') return labels.attendanceRemove;
	return labels.attendanceEdit;
}

export function attendanceApprovalDetailLines(
	detail: AttendanceApprovalDetail,
	labels: AttendanceApprovalLabels,
	findEvent: AttendanceApprovalEventLookup
): string[] {
	if (detail.kind === 'attendance_add') {
		return [
			withLocation(
				`${clockLabel(detail.addition.attendanceKind, labels)} ${detail.addition.localDate} ${detail.addition.localTime}`,
				detail.addition.location
			)
		];
	}
	if (detail.kind === 'attendance_remove') {
		const described = describeEvent(detail.removedEventID, labels, findEvent);
		return described ? [described] : [];
	}
	return detail.corrections.map((correction) => {
		const asRecorded = describeEvent(correction.eventID, labels, findEvent);
		const asRequested = withLocation(
			`${correction.localDate} ${correction.localTime}`,
			correction.location
		);
		return asRecorded ? `${asRecorded} → ${asRequested}` : asRequested;
	});
}

function describeEvent(
	eventID: string,
	labels: AttendanceApprovalLabels,
	findEvent: AttendanceApprovalEventLookup
): string {
	const event = findEvent(eventID);
	if (!event) return '';
	return `${clockLabel(event.kind, labels)} ${event.localDate} ${event.localTime.slice(0, 5)}`;
}

function clockLabel(kind: AttendanceKind, labels: AttendanceApprovalLabels): string {
	return kind === 'clock_out' ? labels.clockOut : labels.clockIn;
}

function withLocation(described: string, location: string): string {
	return location ? `${described} · ${location}` : described;
}
