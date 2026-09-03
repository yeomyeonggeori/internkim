import type {
	EmployeeLeavePayload,
	EmployeeLeavePreview,
	EmployeeLeavePreviewRequest,
	EmployeeLeaveSubmission
} from './employee-leave-types';
import {
	cancelSupabaseLeaveRequest,
	createSupabaseLeaveRequest,
	supabaseEmployeeLeave,
	supabaseLeavePreview
} from '$lib/attendance/supabase-leave';

import { EmployeeLeaveAPIError } from './employee-leave-api-error';

export { EmployeeLeaveAPIError };

export function fetchEmployeeLeave(): Promise<EmployeeLeavePayload> {
	return supabaseEmployeeLeave();
}

export function previewEmployeeLeave(
	request: EmployeeLeavePreviewRequest
): Promise<EmployeeLeavePreview> {
	return supabaseLeavePreview(normalizePreviewRequest(request));
}

export function createEmployeeLeaveRequest(request: EmployeeLeaveSubmission): Promise<void> {
	return createSupabaseLeaveRequest({
		...normalizePreviewRequest(request),
		reason: request.reason.trim()
	});
}

export function cancelEmployeeLeaveRequest(requestID: string): Promise<void> {
	return cancelSupabaseLeaveRequest(requestID);
}

function normalizePreviewRequest(request: EmployeeLeavePreviewRequest): EmployeeLeavePreviewRequest {
	return {
		leaveTypeID: request.leaveTypeID.trim(),
		unit: request.unit,
		startDate: request.startDate.trim(),
		endDate: optionalString(request.endDate),
		partialPeriod: request.partialPeriod,
		startTime: optionalString(request.startTime)
	};
}

function optionalString(value: string | null | undefined): string | undefined {
	const normalized = value?.trim() ?? '';
	return normalized || undefined;
}
