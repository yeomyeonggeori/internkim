import type {
	EmployeeLeaveErrorCode,
	EmployeeLeavePayload,
	EmployeeLeaveSummary
} from './src/routes/attendance/leave/employee-leave-types';
import { buildEmployeeLeaveFixture } from './dev-attendance-leave-fixture';
import {
	cancelEmployeeLeaveRequest,
	createEmployeeLeaveRequest,
	resubmitEmployeeLeaveRequest,
	updateEmployeeLeaveRequest
} from './dev-attendance-leave-lifecycle';
import {
	employeeLeavePreviewRequestFromBody,
	employeeLeaveResubmissionFromRecord,
	employeeLeaveSubmissionFromRecord,
	employeeLeaveUpdateFromRecord,
	multipartPayloadFromRequest
} from './dev-attendance-leave-multipart';
import { buildEmployeeLeavePreview } from './dev-attendance-leave-preview';

export { buildEmployeeLeavePreview };

export type DevEmployeeLeaveMockState = {
	payload: EmployeeLeavePayload;
	nextRequestID: number;
	nextAttachmentID: number;
	nextLedgerID: number;
	managedBalancesByLeaveType: Record<string, EmployeeLeaveSummary | undefined>;
};

export type DevEmployeeLeaveMockRequest = {
	method: string;
	pathname: string;
	body?: string;
	contentType?: string;
};

export type DevEmployeeLeaveMockResponse = {
	status: number;
	body: unknown;
};

export function createDevEmployeeLeaveMockState(): DevEmployeeLeaveMockState {
	const payload = buildEmployeeLeaveFixture();
	return {
		payload,
		nextRequestID: 1,
		nextAttachmentID: 1,
		nextLedgerID: 1,
		managedBalancesByLeaveType: Object.fromEntries(
			payload.leaveTypes.map((leaveType) => [
				leaveType.id,
				leaveType.balance ? structuredClone(leaveType.balance) : undefined
			])
		)
	};
}

export function createDevEmployeeLeaveMockResponse(
	state: DevEmployeeLeaveMockState,
	request: DevEmployeeLeaveMockRequest
): DevEmployeeLeaveMockResponse | undefined {
	if (request.method === 'GET' && request.pathname === '/attendance/api/leave') {
		return { status: 200, body: structuredClone(state.payload) };
	}
	if (request.method === 'POST' && request.pathname === '/attendance/api/leave-requests/preview') {
		return {
			status: 200,
			body: buildEmployeeLeavePreview(employeeLeavePreviewRequestFromBody(request.body))
		};
	}
	if (request.method === 'POST' && request.pathname === '/attendance/api/leave-requests') {
		const multipartPayload = multipartPayloadFromRequest(request);
		const submission = employeeLeaveSubmissionFromRecord(multipartPayload.request);
		const createdRequest = createEmployeeLeaveRequest(
			state,
			submission,
			multipartPayload.attachments
		);
		return { status: 200, body: createdRequest };
	}
	const updateMatch = request.pathname.match(
		/^\/attendance\/api\/leave-requests\/([^/]+)\/update$/
	);
	if (request.method === 'POST' && updateMatch) {
		const requestID = decodeURIComponent(updateMatch[1] ?? '');
		const existingRequest = state.payload.requests.find((request) => request.id === requestID);
		if (!existingRequest) {
			return errorResponse(404, 'requestNotFound', 'leave request not found');
		}
		const multipartPayload = multipartPayloadFromRequest(request);
		const submission = employeeLeaveUpdateFromRecord(multipartPayload.request);
		const updatedRequest = updateEmployeeLeaveRequest(
			state,
			requestID,
			submission,
			multipartPayload.attachments
		);
		if (!updatedRequest) {
			return errorResponse(409, 'invalidStatus', 'leave request cannot be updated');
		}
		return { status: 200, body: updatedRequest };
	}
	const resubmitMatch = request.pathname.match(
		/^\/attendance\/api\/leave-requests\/([^/]+)\/resubmit$/
	);
	if (request.method === 'POST' && resubmitMatch) {
		const requestID = decodeURIComponent(resubmitMatch[1] ?? '');
		const existingRequest = state.payload.requests.find((request) => request.id === requestID);
		if (!existingRequest) {
			return errorResponse(404, 'requestNotFound', 'leave request not found');
		}
		if (!existingRequest.canResubmit) {
			return errorResponse(409, 'invalidStatus', 'leave request cannot be resubmitted');
		}
		const multipartPayload = multipartPayloadFromRequest(request);
		const submission = employeeLeaveResubmissionFromRecord(multipartPayload.request);
		const resubmittedRequest = resubmitEmployeeLeaveRequest(
			state,
			requestID,
			submission,
			multipartPayload.attachments
		);
		if (!resubmittedRequest) {
			return errorResponse(409, 'invalidStatus', 'leave request cannot be resubmitted');
		}
		return { status: 200, body: resubmittedRequest };
	}
	const cancelMatch = request.pathname.match(
		/^\/attendance\/api\/leave-requests\/([^/]+)\/cancel$/
	);
	if (request.method === 'POST' && cancelMatch) {
		const requestID = decodeURIComponent(cancelMatch[1] ?? '');
		const existingRequest = state.payload.requests.find((request) => request.id === requestID);
		if (!existingRequest) {
			return errorResponse(404, 'requestNotFound', 'leave request not found');
		}
		if (!existingRequest.canCancel) {
			return errorResponse(409, 'invalidStatus', 'leave request cannot be cancelled');
		}
		const cancelledRequest = cancelEmployeeLeaveRequest(state, requestID);
		if (!cancelledRequest) {
			return errorResponse(409, 'invalidStatus', 'leave request cannot be cancelled');
		}
		return { status: 200, body: { ok: true } };
	}
	const attachmentMatch = request.pathname.match(
		/^\/attendance\/api\/leave-requests\/([^/]+)\/attachments\/([^/]+)$/
	);
	if (request.method === 'GET' && attachmentMatch) {
		return { status: 200, body: { ok: true } };
	}
	return undefined;
}

function errorResponse(
	status: number,
	code: EmployeeLeaveErrorCode,
	error: string
): DevEmployeeLeaveMockResponse {
	return { status, body: { code, error } };
}
