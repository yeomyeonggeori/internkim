import type {
	EmployeeLeavePreviewRequest,
	EmployeeLeaveResubmission,
	EmployeeLeaveSubmission,
	EmployeeLeaveUpdate
} from './src/routes/attendance/leave/employee-leave-types';

export type DevEmployeeLeaveAttachmentInput = {
	fileName: string;
	contentType: string;
	sizeBytes: number;
};

export type DevEmployeeLeaveMultipartPayload = {
	request: Record<string, unknown>;
	attachments: DevEmployeeLeaveAttachmentInput[];
};

type DevEmployeeLeaveMultipartSource = {
	body?: string;
	contentType?: string;
};

export function multipartPayloadFromRequest(
	request: DevEmployeeLeaveMultipartSource
): DevEmployeeLeaveMultipartPayload {
	const boundary = request.contentType?.match(/boundary="?([^";]+)"?/)?.[1] ?? '';
	if (!boundary || !request.body) return { request: {}, attachments: [] };
	const payload: DevEmployeeLeaveMultipartPayload = { request: {}, attachments: [] };
	for (const part of request.body.split(`--${boundary}`)) {
		const headerEnd = part.indexOf('\r\n\r\n');
		if (headerEnd < 0) continue;
		const headers = part.slice(0, headerEnd);
		const name = headers.match(/name="([^"]+)"/)?.[1];
		const body = part.slice(headerEnd + 4).replace(/\r\n$/, '');
		if (name === 'request') {
			payload.request = parseJSONRecord(body);
			continue;
		}
		if (name !== 'attachments') continue;
		const fileName = headers.match(/filename="([^"]+)"/)?.[1] ?? 'attachment';
		const contentType =
			headers.match(/Content-Type:\s*([^\r\n]+)/i)?.[1]?.trim() ?? 'application/octet-stream';
		payload.attachments.push({
			fileName,
			contentType,
			sizeBytes: new TextEncoder().encode(body).byteLength
		});
	}
	return payload;
}

export function employeeLeavePreviewRequestFromBody(
	body: string | undefined
): EmployeeLeavePreviewRequest {
	return employeeLeavePreviewRequestFromRecord(parseJSONRecord(body));
}

export function employeeLeaveSubmissionFromRecord(
	record: Record<string, unknown>
): EmployeeLeaveSubmission {
	return {
		...employeeLeavePreviewRequestFromRecord(record),
		reason: stringValue(record.reason)
	};
}

export function employeeLeaveResubmissionFromRecord(
	record: Record<string, unknown>
): EmployeeLeaveResubmission {
	const response = stringValue(record.response);
	return {
		...employeeLeaveSubmissionFromRecord(record),
		...(response ? { response } : {})
	};
}

export function employeeLeaveUpdateFromRecord(
	record: Record<string, unknown>
): EmployeeLeaveUpdate {
	return {
		...employeeLeaveSubmissionFromRecord(record),
		revision: numberValue(record.revision),
		removedAttachmentIDs: stringArrayValue(record.removedAttachmentIDs)
	};
}

function employeeLeavePreviewRequestFromRecord(
	record: Record<string, unknown>
): EmployeeLeavePreviewRequest {
	const unit =
		record.unit === 'halfDay' || record.unit === 'quarterDay' ? record.unit : 'fullDay';
	const partialPeriod =
		record.partialPeriod === 'afternoon' || record.partialPeriod === 'custom'
			? record.partialPeriod
			: 'morning';
	const endDate = stringValue(record.endDate);
	const startTime = stringValue(record.startTime);
	return {
		leaveTypeID: stringValue(record.leaveTypeID),
		unit,
		startDate: stringValue(record.startDate),
		...(endDate ? { endDate } : {}),
		...(unit !== 'fullDay' ? { partialPeriod } : {}),
		...(unit !== 'fullDay' && partialPeriod === 'custom' && startTime ? { startTime } : {})
	};
}

function parseJSONRecord(value: string | undefined): Record<string, unknown> {
	if (!value) return {};
	try {
		const parsed: unknown = JSON.parse(value);
		if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
			return parsed as Record<string, unknown>;
		}
	} catch {
		return {};
	}
	return {};
}

function stringValue(value: unknown): string {
	return typeof value === 'string' ? value.trim() : '';
}

function numberValue(value: unknown): number {
	return typeof value === 'number' && Number.isInteger(value) ? value : 0;
}

function stringArrayValue(value: unknown): string[] {
	if (!Array.isArray(value)) return [];
	return value
		.filter((item): item is string => typeof item === 'string')
		.map((item) => item.trim())
		.filter(Boolean);
}
