import type {
	EmployeeLeaveAttachment,
	EmployeeLeaveRequest,
	EmployeeLeaveResubmission,
	EmployeeLeaveSubmission,
	EmployeeLeaveUpdate
} from './src/routes/attendance/leave/employee-leave-types';
import type { DevEmployeeLeaveMockState } from './dev-attendance-leave-mock';
import type { DevEmployeeLeaveAttachmentInput } from './dev-attendance-leave-multipart';
import { buildEmployeeLeavePreview } from './dev-attendance-leave-preview';

export function createEmployeeLeaveRequest(
	state: DevEmployeeLeaveMockState,
	submission: EmployeeLeaveSubmission,
	attachmentInputs: DevEmployeeLeaveAttachmentInput[]
): EmployeeLeaveRequest {
	const preview = buildEmployeeLeavePreview(submission);
	const leaveType = state.payload.leaveTypes.find(
		(candidate) => candidate.id === submission.leaveTypeID
	);
	const requestID = `leave-request-created-${state.nextRequestID++}`;
	const createdAt = new Date().toISOString();
	const request: EmployeeLeaveRequest = {
		id: requestID,
		leaveTypeID: submission.leaveTypeID,
		leaveTypeName: leaveType?.name ?? submission.leaveTypeID,
		status: 'pending',
		unit: submission.unit,
		startDate: submission.startDate,
		...(submission.endDate ? { endDate: submission.endDate } : {}),
		...(submission.partialPeriod ? { partialPeriod: submission.partialPeriod } : {}),
		...(preview.occurrences[0]
			? {
					startTime: preview.occurrences[0].startTime,
					endTime: preview.occurrences.at(-1)?.endTime
				}
			: {}),
		deductionMilliDays: preview.totalDeductionMilliDays,
		reason: submission.reason,
		attachments: buildAttachments(state, requestID, attachmentInputs),
		canCancel: true,
		canEdit: true,
		canResubmit: false,
		revision: 1,
		createdAt
	};
	state.payload.requests.unshift(request);
	reserveBalance(state, request, preview.totalDeductionMilliDays, createdAt);
	return structuredClone(request);
}

export function resubmitEmployeeLeaveRequest(
	state: DevEmployeeLeaveMockState,
	requestID: string,
	submission: EmployeeLeaveResubmission,
	attachmentInputs: DevEmployeeLeaveAttachmentInput[]
): EmployeeLeaveRequest | undefined {
	const requestIndex = state.payload.requests.findIndex((request) => request.id === requestID);
	const previousRequest = state.payload.requests[requestIndex];
	if (!previousRequest || !previousRequest.canResubmit) return undefined;
	const preview = buildEmployeeLeavePreview(submission);
	const leaveType = state.payload.leaveTypes.find(
		(candidate) => candidate.id === submission.leaveTypeID
	);
	const updatedAt = new Date().toISOString();
	const nextRequest: EmployeeLeaveRequest = {
		...previousRequest,
		leaveTypeID: submission.leaveTypeID,
		leaveTypeName: leaveType?.name ?? submission.leaveTypeID,
		status: 'pending',
		unit: submission.unit,
		startDate: submission.startDate,
		endDate: submission.endDate,
		partialPeriod: submission.partialPeriod,
		startTime: preview.occurrences[0]?.startTime,
		endTime: preview.occurrences.at(-1)?.endTime,
		deductionMilliDays: preview.totalDeductionMilliDays,
		reason: submission.reason,
		attachments: [
			...previousRequest.attachments,
			...buildAttachments(state, requestID, attachmentInputs)
		],
		canCancel: true,
		canEdit: true,
		canResubmit: false,
		revision: previousRequest.revision + 1,
		updatedAt
	};
	state.payload.requests[requestIndex] = nextRequest;
	const deductionDifference =
		preview.totalDeductionMilliDays - previousRequest.deductionMilliDays;
	if (deductionDifference !== 0) {
		reserveBalance(state, nextRequest, deductionDifference, updatedAt);
	}
	return structuredClone(nextRequest);
}

export function updateEmployeeLeaveRequest(
	state: DevEmployeeLeaveMockState,
	requestID: string,
	submission: EmployeeLeaveUpdate,
	attachmentInputs: DevEmployeeLeaveAttachmentInput[]
): EmployeeLeaveRequest | undefined {
	const requestIndex = state.payload.requests.findIndex((request) => request.id === requestID);
	const previousRequest = state.payload.requests[requestIndex];
	if (
		!previousRequest ||
		!previousRequest.canEdit ||
		previousRequest.revision !== submission.revision
	) {
		return undefined;
	}
	const preview = buildEmployeeLeavePreview(submission);
	const leaveType = state.payload.leaveTypes.find(
		(candidate) => candidate.id === submission.leaveTypeID
	);
	const updatedAt = new Date().toISOString();
	const removedAttachmentIDs = new Set(submission.removedAttachmentIDs);
	const nextRequest: EmployeeLeaveRequest = {
		...previousRequest,
		leaveTypeID: submission.leaveTypeID,
		leaveTypeName: leaveType?.name ?? submission.leaveTypeID,
		unit: submission.unit,
		startDate: submission.startDate,
		endDate: submission.endDate,
		partialPeriod: submission.partialPeriod,
		startTime: preview.occurrences[0]?.startTime,
		endTime: preview.occurrences.at(-1)?.endTime,
		deductionMilliDays: preview.totalDeductionMilliDays,
		reason: submission.reason,
		attachments: [
			...previousRequest.attachments.filter(
				(attachment) => !removedAttachmentIDs.has(attachment.id)
			),
			...buildAttachments(state, requestID, attachmentInputs)
		],
		revision: previousRequest.revision + 1,
		updatedAt
	};
	releaseBalance(state, previousRequest, updatedAt);
	state.payload.requests[requestIndex] = nextRequest;
	reserveBalance(state, nextRequest, preview.totalDeductionMilliDays, updatedAt);
	return structuredClone(nextRequest);
}

export function cancelEmployeeLeaveRequest(
	state: DevEmployeeLeaveMockState,
	requestID: string
): EmployeeLeaveRequest | undefined {
	const requestIndex = state.payload.requests.findIndex((request) => request.id === requestID);
	const previousRequest = state.payload.requests[requestIndex];
	if (!previousRequest || !previousRequest.canCancel) return undefined;
	const updatedAt = new Date().toISOString();
	if (previousRequest.status === 'pending' || previousRequest.status === 'needsChanges') {
		state.payload.requests.splice(requestIndex, 1);
		releaseBalance(state, previousRequest, updatedAt);
		for (const ledgerEntry of state.payload.ledgerEntries) {
			if (ledgerEntry.requestID === requestID) {
				ledgerEntry.requestID = undefined;
			}
		}
		return structuredClone(previousRequest);
	}
	if (previousRequest.status !== 'approved') return undefined;
	const nextRequest: EmployeeLeaveRequest = {
		...previousRequest,
		status: 'cancelled',
		canCancel: false,
		canEdit: false,
		canResubmit: false,
		updatedAt
	};
	state.payload.requests[requestIndex] = nextRequest;
	restoreUsedBalance(state, nextRequest, updatedAt);
	return structuredClone(nextRequest);
}

function reserveBalance(
	state: DevEmployeeLeaveMockState,
	request: EmployeeLeaveRequest,
	deductionMilliDays: number,
	occurredAt: string
): void {
	const leaveType = state.payload.leaveTypes.find(
		(candidate) => candidate.id === request.leaveTypeID
	);
	if (leaveType?.balanceMode === 'none' || deductionMilliDays === 0) return;
	state.payload.summary.reservedMilliDays += deductionMilliDays;
	state.payload.summary.availableMilliDays -= deductionMilliDays;
	state.payload.ledgerEntries.unshift({
		id: `leave-ledger-created-${state.nextLedgerID++}`,
		operationKey: `reserve:${request.id}:${occurredAt}`,
		operationType: 'reserve',
		occurredAt,
		leaveTypeID: request.leaveTypeID,
		leaveTypeName: request.leaveTypeName,
		deltaMilliDays: -deductionMilliDays,
		balanceAfterMilliDays: state.payload.summary.availableMilliDays,
		isUntracked: false,
		requestID: request.id
	});
}

function releaseBalance(
	state: DevEmployeeLeaveMockState,
	request: EmployeeLeaveRequest,
	occurredAt: string
): void {
	const leaveType = state.payload.leaveTypes.find(
		(candidate) => candidate.id === request.leaveTypeID
	);
	if (leaveType?.balanceMode === 'none' || request.deductionMilliDays === 0) return;
	state.payload.summary.reservedMilliDays -= request.deductionMilliDays;
	state.payload.summary.availableMilliDays += request.deductionMilliDays;
	state.payload.ledgerEntries.unshift({
		id: `leave-ledger-created-${state.nextLedgerID++}`,
		operationKey: `release:${request.id}:${occurredAt}`,
		operationType: 'release',
		occurredAt,
		leaveTypeID: request.leaveTypeID,
		leaveTypeName: request.leaveTypeName,
		deltaMilliDays: request.deductionMilliDays,
		balanceAfterMilliDays: state.payload.summary.availableMilliDays,
		isUntracked: false,
		requestID: request.id
	});
}

function restoreUsedBalance(
	state: DevEmployeeLeaveMockState,
	request: EmployeeLeaveRequest,
	occurredAt: string
): void {
	const leaveType = state.payload.leaveTypes.find(
		(candidate) => candidate.id === request.leaveTypeID
	);
	if (leaveType?.balanceMode === 'none' || request.deductionMilliDays === 0) return;
	state.payload.summary.usedMilliDays -= request.deductionMilliDays;
	state.payload.summary.availableMilliDays += request.deductionMilliDays;
	state.payload.ledgerEntries.unshift({
		id: `leave-ledger-created-${state.nextLedgerID++}`,
		operationKey: `restore:${request.id}:${occurredAt}`,
		operationType: 'restore',
		occurredAt,
		leaveTypeID: request.leaveTypeID,
		leaveTypeName: request.leaveTypeName,
		deltaMilliDays: request.deductionMilliDays,
		balanceAfterMilliDays: state.payload.summary.availableMilliDays,
		isUntracked: false,
		requestID: request.id
	});
}

function buildAttachments(
	state: DevEmployeeLeaveMockState,
	requestID: string,
	inputs: DevEmployeeLeaveAttachmentInput[]
): EmployeeLeaveAttachment[] {
	return inputs.map((input) => {
		const attachmentID = `leave-attachment-created-${state.nextAttachmentID++}`;
		return {
			id: attachmentID,
			fileName: input.fileName,
			contentType: input.contentType,
			sizeBytes: input.sizeBytes,
			downloadURL: `/attendance/api/leave-requests/${encodeURIComponent(requestID)}/attachments/${encodeURIComponent(attachmentID)}`
		};
	});
}
