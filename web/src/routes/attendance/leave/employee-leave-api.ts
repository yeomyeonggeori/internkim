import type {
	EmployeeLeaveErrorCode,
	EmployeeLeavePayload,
	EmployeeLeavePreview,
	EmployeeLeavePreviewRequest,
	EmployeeLeaveRequest,
	EmployeeLeaveResubmission,
	EmployeeLeaveSubmission,
	EmployeeLeaveUpdate
} from './employee-leave-types';
import { isEmployeeLeaveErrorCode } from './employee-leave-types';

export class EmployeeLeaveAPIError extends Error {
	constructor(
		readonly code: EmployeeLeaveErrorCode | null,
		readonly status: number
	) {
		super('Employee leave API request failed');
		this.name = 'EmployeeLeaveAPIError';
	}
}

export async function fetchEmployeeLeave(): Promise<EmployeeLeavePayload> {
	const response = await fetch('/attendance/api/leave', {
		credentials: 'include',
		cache: 'no-store'
	});
	const payload = await readJSON<EmployeeLeavePayload>(response);
	return {
		...payload,
		leaveTypes: (payload.leaveTypes ?? []).map((leaveType) => ({
			...leaveType,
			requiresHireDate: leaveType.requiresHireDate === true
		})),
		requests: (payload.requests ?? []).map(normalizeEmployeeLeaveRequest),
		ledgerEntries: payload.ledgerEntries ?? [],
		hireDateRequired: payload.hireDateRequired === true
	};
}

export async function previewEmployeeLeave(
	request: EmployeeLeavePreviewRequest
): Promise<EmployeeLeavePreview> {
	const response = await fetch('/attendance/api/leave-requests/preview', {
		method: 'POST',
		credentials: 'include',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(normalizePreviewRequest(request))
	});
	return readJSON<EmployeeLeavePreview>(response);
}

export async function createEmployeeLeaveRequest(
	request: EmployeeLeaveSubmission,
	attachments: File[]
): Promise<void> {
	const response = await fetch('/attendance/api/leave-requests', {
		method: 'POST',
		credentials: 'include',
		body: employeeLeaveFormData(request, attachments)
	});
	if (!response.ok) throw await failedResponseError(response);
}

export async function resubmitEmployeeLeaveRequest(
	requestID: string,
	request: EmployeeLeaveResubmission,
	attachments: File[]
): Promise<void> {
	const response = await fetch(
		`/attendance/api/leave-requests/${encodeURIComponent(requestID)}/resubmit`,
		{
			method: 'POST',
			credentials: 'include',
			body: employeeLeaveFormData(request, attachments)
		}
	);
	if (!response.ok) throw await failedResponseError(response);
}

export async function updateEmployeeLeaveRequest(
	requestID: string,
	request: EmployeeLeaveUpdate,
	attachments: File[]
): Promise<void> {
	const response = await fetch(
		`/attendance/api/leave-requests/${encodeURIComponent(requestID)}/update`,
		{
			method: 'POST',
			credentials: 'include',
			body: employeeLeaveFormData(request, attachments)
		}
	);
	if (!response.ok) throw await failedResponseError(response);
}

export async function cancelEmployeeLeaveRequest(requestID: string): Promise<void> {
	const response = await fetch(
		`/attendance/api/leave-requests/${encodeURIComponent(requestID)}/cancel`,
		{
			method: 'POST',
			credentials: 'include'
		}
	);
	if (!response.ok) throw await failedResponseError(response);
}

export function employeeLeaveAttachmentURL(requestID: string, attachmentID: string): string {
	return `/attendance/api/leave-requests/${encodeURIComponent(requestID)}/attachments/${encodeURIComponent(attachmentID)}`;
}

function employeeLeaveFormData(
	request: EmployeeLeaveSubmission | EmployeeLeaveResubmission | EmployeeLeaveUpdate,
	attachments: File[]
): FormData {
	const formData = new FormData();
	formData.append('request', JSON.stringify(normalizeSubmission(request)));
	for (const attachment of attachments) {
		formData.append('attachments', attachment, attachment.name);
	}
	return formData;
}

function normalizeSubmission(
	request: EmployeeLeaveSubmission | EmployeeLeaveResubmission | EmployeeLeaveUpdate
): EmployeeLeaveSubmission | EmployeeLeaveResubmission | EmployeeLeaveUpdate {
	return compactOptionalStrings({
		...normalizePreviewRequest(request),
		reason: request.reason.trim(),
		...('response' in request ? { response: optionalString(request.response) } : {}),
		...('revision' in request
			? {
					revision: request.revision,
					removedAttachmentIDs: request.removedAttachmentIDs
				}
			: {})
	}) as EmployeeLeaveSubmission | EmployeeLeaveResubmission | EmployeeLeaveUpdate;
}

function normalizePreviewRequest(request: EmployeeLeavePreviewRequest): EmployeeLeavePreviewRequest {
	return compactOptionalStrings({
		leaveTypeID: request.leaveTypeID.trim(),
		unit: request.unit,
		startDate: request.startDate.trim(),
		endDate: optionalString(request.endDate),
		partialPeriod: request.partialPeriod,
		startTime: optionalString(request.startTime)
	}) as EmployeeLeavePreviewRequest;
}

function normalizeEmployeeLeaveRequest(request: EmployeeLeaveRequest): EmployeeLeaveRequest {
	return {
		...request,
		endDate: optionalString(request.endDate),
		partialPeriod: request.partialPeriod || undefined,
		startTime: optionalString(request.startTime),
		endTime: optionalString(request.endTime),
		adminResponse: optionalString(request.adminResponse),
		updatedAt: optionalString(request.updatedAt),
		attachments: request.attachments ?? [],
		canEdit: request.canEdit === true,
		revision: Number.isInteger(request.revision) ? request.revision : 0
	};
}

function compactOptionalStrings(value: Record<string, unknown>): Record<string, unknown> {
	return Object.fromEntries(
		Object.entries(value).filter(([, fieldValue]) => fieldValue !== undefined && fieldValue !== '')
	);
}

function optionalString(value: string | null | undefined): string | undefined {
	const normalized = value?.trim() ?? '';
	return normalized || undefined;
}

async function readJSON<Value>(response: Response): Promise<Value> {
	if (!response.ok) throw await failedResponseError(response);
	return (await response.json()) as Value;
}

async function failedResponseError(response: Response): Promise<EmployeeLeaveAPIError> {
	const parsed: unknown = await response.json().catch(() => null);
	let code: EmployeeLeaveErrorCode | null = null;
	if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
		const candidateCode = (parsed as Record<string, unknown>).code;
		if (isEmployeeLeaveErrorCode(candidateCode)) code = candidateCode;
	}
	return new EmployeeLeaveAPIError(code, response.status);
}
