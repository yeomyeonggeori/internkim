import type {
	LeaveApprovalDecision,
	LeaveApprovalInbox,
	LeaveApprovalRequest
} from './leave-approval-types';

export class LeaveApprovalAPIError extends Error {
	constructor(
		readonly code: string | null,
		readonly status: number
	) {
		super('Leave approval API request failed');
		this.name = 'LeaveApprovalAPIError';
	}
}

export async function fetchLeaveApprovalInbox(): Promise<LeaveApprovalInbox> {
	const response = await fetch('/attendance/api/leave-approvals', {
		credentials: 'include',
		cache: 'no-store'
	});
	const inbox = await readJSON<LeaveApprovalInbox>(response);
	return {
		pendingCount: inbox.pendingCount ?? 0,
		pending: inbox.pending ?? [],
		recentChanges: inbox.recentChanges ?? []
	};
}

export async function decideLeaveApproval(
	requestID: string,
	decision: LeaveApprovalDecision
): Promise<LeaveApprovalRequest> {
	const response = await fetch(
		`/attendance/api/leave-approvals/${encodeURIComponent(requestID)}`,
		{
			method: 'POST',
			credentials: 'include',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({
				action: decision.action,
				response: decision.response?.trim() ?? ''
			})
		}
	);
	const payload = await readJSON<{ request: LeaveApprovalRequest }>(response);
	return payload.request;
}

async function readJSON<Value>(response: Response): Promise<Value> {
	if (!response.ok) throw await failedResponseError(response);
	return (await response.json()) as Value;
}

async function failedResponseError(response: Response): Promise<LeaveApprovalAPIError> {
	const parsed: unknown = await response.json().catch(() => null);
	const code =
		parsed && typeof parsed === 'object' && !Array.isArray(parsed)
			? stringValue((parsed as Record<string, unknown>).code)
			: null;
	return new LeaveApprovalAPIError(code, response.status);
}

function stringValue(value: unknown): string | null {
	return typeof value === 'string' && value.trim() ? value : null;
}
