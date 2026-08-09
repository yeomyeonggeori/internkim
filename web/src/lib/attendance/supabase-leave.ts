import { supabase } from '$lib/supabase';
import type {
	EmployeeLeavePayload,
	EmployeeLeavePreview,
	EmployeeLeavePreviewRequest,
	EmployeeLeaveRequest,
	EmployeeLeaveStatus,
	EmployeeLeaveSubmission,
	EmployeeLeaveType,
	EmployeeLeaveUnit
} from '../../routes/attendance/leave/employee-leave-types';
import type {
	LeaveApprovalDecision,
	LeaveApprovalInbox,
	LeaveApprovalRequest
} from '../../routes/attendance/approval/leave-approval-types';

type LeaveStatus = 'requested' | 'approved' | 'rejected';

type LeaveRow = {
	id: string;
	member_id: string;
	kind: string;
	is_paid: boolean;
	days: number;
	status: LeaveStatus;
	starts_at: string;
	ends_at: string;
	note: string | null;
};

const statusWords: Record<LeaveStatus, EmployeeLeaveStatus> = {
	requested: 'pending',
	approved: 'approved',
	rejected: 'rejected'
};

const dayInMilliseconds = 24 * 60 * 60 * 1000;

const theOnlyLeaveType: EmployeeLeaveType = {
	id: 'leave',
	name: '휴가',
	balanceMode: 'none',
	allowedUnits: ['fullDay', 'halfDay', 'quarterDay'],
	includeInSummary: false,
	isActive: true,
	requiresHireDate: false
};

export async function supabaseEmployeeLeave(): Promise<EmployeeLeavePayload> {
	const memberID = await myMemberID();
	const leave = await supabase()
		.from('leave')
		.select('id, member_id, kind, is_paid, days, status, starts_at, ends_at, note')
		.eq('member_id', memberID)
		.order('starts_at', { ascending: false })
		.returns<LeaveRow[]>();
	if (leave.error) throw new Error(leave.error.message);

	return {
		balanceTrackingMode: 'unlimited',
		leaveTypes: [theOnlyLeaveType],
		summary: { usedMilliDays: 0, reservedMilliDays: 0, availableMilliDays: 0 },
		requests: leave.data.map(requestOf),
		ledgerEntries: [],
		hireDateRequired: false
	};
}

export async function supabaseLeavePreview(request: EmployeeLeavePreviewRequest): Promise<EmployeeLeavePreview> {
	const startDate = request.startDate;
	const endDate = request.endDate || startDate;
	const perDay = deductionOf(request.unit);
	const occurrences = [];
	for (let date = startDate; date <= endDate; date = shiftedDay(date, 1)) {
		occurrences.push({ date, startTime: request.startTime ?? '', endTime: '', deductionMilliDays: perDay });
	}
	return {
		occurrences,
		excludedDates: [],
		totalDeductionMilliDays: occurrences.length * perDay
	};
}

export async function createSupabaseLeaveRequest(request: EmployeeLeaveSubmission): Promise<void> {
	const preview = await supabaseLeavePreview(request);
	const { error } = await supabase().from('leave').insert({
		member_id: await myMemberID(),
		kind: theOnlyLeaveType.id,
		is_paid: true,
		days: preview.totalDeductionMilliDays / 1000,
		status: 'requested',
		starts_at: new Date(`${request.startDate}T00:00:00Z`).toISOString(),
		ends_at: new Date(`${shiftedDay(request.endDate || request.startDate, 1)}T00:00:00Z`).toISOString(),
		note: request.reason
	});
	if (error) throw new Error(error.message);
}

export async function cancelSupabaseLeaveRequest(requestID: string): Promise<void> {
	const { error } = await supabase().from('leave').delete().eq('id', requestID);
	if (error) throw new Error(error.message);
}

export async function supabaseLeaveApprovalInbox(): Promise<LeaveApprovalInbox> {
	const leave = await supabase()
		.from('leave')
		.select('id, member_id, kind, is_paid, days, status, starts_at, ends_at, note')
		.eq('status', 'requested')
		.order('starts_at')
		.returns<LeaveRow[]>();
	if (leave.error) throw new Error(leave.error.message);

	const emails = await emailsByMemberID();
	const pending = leave.data.map((row) => approvalOf(row, emails));
	return { pendingCount: pending.length, pending, recentChanges: [] };
}

export async function decideSupabaseLeave(
	requestID: string,
	decision: LeaveApprovalDecision
): Promise<LeaveApprovalRequest> {
	const decided = await supabase()
		.from('leave')
		.update({ status: decision.action === 'approve' ? 'approved' : 'rejected' })
		.eq('id', requestID)
		.select('id, member_id, kind, is_paid, days, status, starts_at, ends_at, note')
		.single<LeaveRow>();
	if (decided.error) throw new Error(decided.error.message);
	return approvalOf(decided.data, await emailsByMemberID());
}

function requestOf(row: LeaveRow): EmployeeLeaveRequest {
	const status = statusWords[row.status];
	return {
		id: row.id,
		leaveTypeID: theOnlyLeaveType.id,
		leaveTypeName: theOnlyLeaveType.name,
		status,
		unit: unitOf(row.days),
		startDate: row.starts_at.slice(0, 10),
		endDate: lastDayOf(row),
		deductionMilliDays: Math.round(row.days * 1000),
		reason: row.note ?? '',
		attachments: [],
		canCancel: status === 'pending',
		canEdit: false,
		canResubmit: false,
		revision: 0,
		createdAt: row.starts_at
	};
}

function approvalOf(row: LeaveRow, emails: Map<string, string>): LeaveApprovalRequest {
	return {
		id: row.id,
		employeeEmail: emails.get(row.member_id) ?? '',
		leaveTypeID: theOnlyLeaveType.id,
		leaveTypeName: theOnlyLeaveType.name,
		balanceMode: 'none',
		status: statusWords[row.status],
		unit: unitOf(row.days),
		startDate: row.starts_at.slice(0, 10),
		endDate: lastDayOf(row),
		deductionMilliDays: Math.round(row.days * 1000),
		reason: row.note ?? '',
		attachments: [],
		balance: { usedMilliDays: 0, reservedMilliDays: 0, availableMilliDays: 0 },
		createdAt: row.starts_at,
		updatedAt: row.starts_at
	};
}

function lastDayOf(row: LeaveRow): string {
	return new Date(new Date(row.ends_at).getTime() - dayInMilliseconds).toISOString().slice(0, 10);
}

function unitOf(days: number): EmployeeLeaveUnit {
	if (days <= 0.25) return 'quarterDay';
	if (days <= 0.5) return 'halfDay';
	return 'fullDay';
}

function deductionOf(unit: EmployeeLeaveUnit): number {
	if (unit === 'quarterDay') return 250;
	if (unit === 'halfDay') return 500;
	return 1000;
}

function shiftedDay(date: string, days: number): string {
	const moved = new Date(`${date}T00:00:00Z`);
	moved.setUTCDate(moved.getUTCDate() + days);
	return moved.toISOString().slice(0, 10);
}

async function myMemberID(): Promise<string> {
	const client = supabase();
	const { data: auth } = await client.auth.getSession();
	const accountID = auth.session?.user.id;
	if (!accountID) throw new Error('sign in first');
	const member = await client.from('member').select('id').eq('user_id', accountID).single<{ id: string }>();
	if (member.error) throw new Error(member.error.message);
	return member.data.id;
}

async function emailsByMemberID(): Promise<Map<string, string>> {
	const members = await supabase()
		.from('member')
		.select('id, email')
		.returns<{ id: string; email: string | null }[]>();
	if (members.error) throw new Error(members.error.message);
	return new Map(members.data.map((member) => [member.id, member.email ?? '']));
}
