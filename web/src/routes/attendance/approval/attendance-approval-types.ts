import type { AttendanceKind } from '../attendance-context.svelte';

export type AttendanceApprovalKind = 'attendance_add' | 'attendance_edit' | 'attendance_remove';
export type AttendanceApprovalDecision = 'approved' | 'rejected';

export type AttendanceApprovalAddition = {
	attendanceKind: AttendanceKind;
	localDate: string;
	localTime: string;
	location: string;
};

export type AttendanceApprovalCorrection = {
	eventID: string;
	localDate: string;
	localTime: string;
	location: string;
};

export type AttendanceApprovalDetail =
	| { kind: 'attendance_add'; addition: AttendanceApprovalAddition }
	| { kind: 'attendance_edit'; corrections: AttendanceApprovalCorrection[] }
	| { kind: 'attendance_remove'; removedEventID: string };

export type AttendanceApprovalRequest = {
	id: string;
	memberID: string;
	askedBy: string;
	reason: string;
	createdAt: string;
	detail: AttendanceApprovalDetail;
};

export type AttendanceApprovalRow = {
	id: string;
	member_id: string;
	asked_by: string | null;
	kind: string;
	payload: unknown;
	reason: string;
	created_at: string;
};

export function attendanceApprovalRequestFrom(row: AttendanceApprovalRow): AttendanceApprovalRequest {
	return {
		id: row.id,
		memberID: row.member_id,
		askedBy: row.asked_by ?? '',
		reason: row.reason,
		createdAt: row.created_at,
		detail: detailFrom(row.kind, objectFrom(row.payload))
	};
}

function detailFrom(kind: string, payload: Record<string, unknown>): AttendanceApprovalDetail {
	if (kind === 'attendance_add') return { kind, addition: additionFrom(payload) };
	if (kind === 'attendance_remove') {
		return { kind, removedEventID: stringFrom(payload.eventID) };
	}
	if (kind === 'attendance_edit') {
		return { kind, corrections: correctionsFrom(payload.corrections) };
	}
	throw new Error(`no approval request goes by the kind ${kind}`);
}

function additionFrom(payload: Record<string, unknown>): AttendanceApprovalAddition {
	return {
		attendanceKind: payload.kind === 'clock_out' ? 'clock_out' : 'clock_in',
		localDate: stringFrom(payload.localDate),
		localTime: shortTime(stringFrom(payload.localTime)),
		location: stringFrom(payload.location)
	};
}

function correctionsFrom(value: unknown): AttendanceApprovalCorrection[] {
	if (!Array.isArray(value)) return [];
	return value.map((entry) => {
		const correction = objectFrom(entry);
		return {
			eventID: stringFrom(correction.event_id),
			localDate: stringFrom(correction.local_date),
			localTime: shortTime(stringFrom(correction.local_time)),
			location: stringFrom(correction.location)
		};
	});
}

function objectFrom(value: unknown): Record<string, unknown> {
	if (!value || typeof value !== 'object' || Array.isArray(value)) return {};
	return value as Record<string, unknown>;
}

function stringFrom(value: unknown): string {
	return typeof value === 'string' ? value : '';
}

function shortTime(localTime: string): string {
	return localTime.slice(0, 5);
}
