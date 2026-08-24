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
import {
	leaveDisplayRange,
	leavePreviewPeriod,
	leaveTimestampRange
} from './supabase-leave-range';
import { EmployeeLeaveAPIError } from '../../routes/attendance/leave/employee-leave-api-error';
import { summarizeSupabaseLeave } from './supabase-leave-summary';

type LeaveStatus = 'requested' | 'approved' | 'rejected';

type MemberDirectory = {
	emailOf: (memberID: string) => string;
	timeZoneOf: (memberID: string) => string;
};

export type LeaveRow = {
	id: string;
	member_id: string;
	kind: string;
	is_paid: boolean;
	is_deducted: boolean;
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

export const theOnlyLeaveType: EmployeeLeaveType = {
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
		.select('id, member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at, note')
		.eq('member_id', memberID)
		.order('starts_at', { ascending: false })
		.returns<LeaveRow[]>();
	if (leave.error) throw new Error(leave.error.message);
	const timeZone = await memberTimeZone(memberID);
	const mappedLeave = leave.data.map((row) => ({ row, request: employeeLeaveRequestOfRow(row, timeZone) }));
	const requests = mappedLeave.map(({ request }) => request);
	const targetYear = await memberCurrentYear(memberID);
	const remainingDays = await memberLeaveRemaining(memberID, targetYear);
	const balance = summarizeSupabaseLeave(
		mappedLeave.map(({ row, request }) => ({
			days: row.days,
			status: row.status,
			isDeducted: row.is_deducted,
			localStartDate: request.startDate,
			localEndDate: request.endDate || request.startDate
		})),
		targetYear,
		remainingDays
	);
	const leaveType: EmployeeLeaveType = {
		...theOnlyLeaveType,
		balanceMode: balance.trackingMode === 'managed' ? 'annual' : 'none',
		includeInSummary: true,
		balance: balance.summary
	};

	return {
		balanceTrackingMode: balance.trackingMode,
		leaveTypes: [leaveType],
		summary: balance.summary,
		requests,
		ledgerEntries: [],
		hireDateRequired: false
	};
}

export async function supabaseLeavePreview(request: EmployeeLeavePreviewRequest): Promise<EmployeeLeavePreview> {
	const startDate = request.startDate;
	const endDate = request.endDate || startDate;
	const period = leavePreviewPeriod(request);
	const occurrences = [];
	for (let date = startDate; date <= endDate; date = shiftedDay(date, 1)) {
		occurrences.push({ date, ...period });
	}
	return {
		occurrences,
		excludedDates: [],
		totalDeductionMilliDays: occurrences.length * period.deductionMilliDays
	};
}

export async function createSupabaseLeaveRequest(request: EmployeeLeaveSubmission): Promise<void> {
	const preview = await supabaseLeavePreview(request);
	const range = leaveTimestampRange(request, await companyTimeZone());
	const { error } = await supabase().from('leave').insert({
		member_id: await myMemberID(),
		kind: theOnlyLeaveType.id,
		is_paid: true,
		days: preview.totalDeductionMilliDays / 1000,
		status: 'requested',
		starts_at: range.startsAt,
		ends_at: range.endsAt,
		note: request.reason
	});
	if (error) throw new Error(error.message);
}

export async function cancelSupabaseLeaveRequest(requestID: string): Promise<void> {
	const withdrawn = await supabase()
		.from('leave')
		.delete()
		.eq('id', requestID)
		.select('id')
		.returns<{ id: string }[]>();
	if (withdrawn.error) throw new Error(withdrawn.error.message);
	if (withdrawn.data.length === 0) throw new EmployeeLeaveAPIError('invalidStatus', 409);
}

export async function supabaseLeaveApprovalInbox(): Promise<LeaveApprovalInbox> {
	const leave = await supabase()
		.from('leave')
		.select('id, member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at, note')
		.eq('status', 'requested')
		.order('starts_at')
		.returns<LeaveRow[]>();
	if (leave.error) throw new Error(leave.error.message);

	const directory = await memberDirectory();
	const pending = leave.data.map((row) => approvalOf(row, directory));
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
		.select('id, member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at, note')
		.single<LeaveRow>();
	if (decided.error) throw new Error(decided.error.message);
	return approvalOf(decided.data, await memberDirectory());
}

export function employeeLeaveRequestOfRow(row: LeaveRow, timeZone: string): EmployeeLeaveRequest {
	const status = statusWords[row.status];
	const range = leaveDisplayRange(row.starts_at, row.ends_at, row.days, timeZone);
	return {
		id: row.id,
		leaveTypeID: theOnlyLeaveType.id,
		leaveTypeName: theOnlyLeaveType.name,
		status,
		unit: unitOf(row.days),
		...range,
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

function approvalOf(row: LeaveRow, directory: MemberDirectory): LeaveApprovalRequest {
	const timeZone = directory.timeZoneOf(row.member_id);
	return {
		id: row.id,
		employeeEmail: directory.emailOf(row.member_id),
		leaveTypeID: theOnlyLeaveType.id,
		leaveTypeName: theOnlyLeaveType.name,
		balanceMode: 'none',
		status: statusWords[row.status],
		unit: unitOf(row.days),
		...leaveDisplayRange(row.starts_at, row.ends_at, row.days, timeZone),
		deductionMilliDays: Math.round(row.days * 1000),
		reason: row.note ?? '',
		attachments: [],
		balance: { usedMilliDays: 0, reservedMilliDays: 0, availableMilliDays: 0 },
		createdAt: row.starts_at,
		updatedAt: row.starts_at
	};
}

function unitOf(days: number): EmployeeLeaveUnit {
	if (days <= 0.25) return 'quarterDay';
	if (days <= 0.5) return 'halfDay';
	return 'fullDay';
}

function shiftedDay(date: string, days: number): string {
	const moved = new Date(`${date}T00:00:00Z`);
	moved.setUTCDate(moved.getUTCDate() + days);
	return moved.toISOString().slice(0, 10);
}

export async function memberTimeZone(memberID: string): Promise<string> {
	const member = await supabase()
		.from('member')
		.select('timezone')
		.eq('id', memberID)
		.single<{ timezone: string | null }>();
	if (member.error) throw new Error(member.error.message);
	return member.data.timezone || (await companyTimeZone());
}

export async function companyTimeZone(): Promise<string> {
	const company = await supabase().from('company').select('timezone').limit(1).single<{ timezone: string }>();
	if (company.error) throw new Error(company.error.message);
	return company.data.timezone;
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

async function memberCurrentYear(memberID: string): Promise<number> {
	const today = await supabase().rpc('member_today', { target_member: memberID });
	if (today.error) throw new Error(today.error.message);
	if (typeof today.data !== 'string') throw new Error('member_today returned an invalid date');
	const year = Number(today.data.slice(0, 4));
	if (!Number.isInteger(year)) throw new Error(`member_today returned an invalid date: ${today.data}`);
	return year;
}

async function memberLeaveRemaining(memberID: string, targetYear: number): Promise<number | null> {
	const remaining = await supabase().rpc('member_leave_remaining', {
		target_member: memberID,
		target_year: targetYear
	});
	if (remaining.error) throw new Error(remaining.error.message);
	if (remaining.data === null) return null;
	const days = Number(remaining.data);
	if (!Number.isFinite(days)) {
		throw new Error(`member_leave_remaining returned an invalid balance: ${String(remaining.data)}`);
	}
	return days;
}

async function memberDirectory(): Promise<MemberDirectory> {
	const companyZone = await companyTimeZone();
	const members = await supabase()
		.from('member')
		.select('id, email, timezone')
		.returns<{ id: string; email: string | null; timezone: string | null }[]>();
	if (members.error) throw new Error(members.error.message);
	const emails = new Map(members.data.map((member) => [member.id, member.email ?? '']));
	const timeZones = new Map(
		members.data.map((member) => [member.id, member.timezone || companyZone])
	);
	return {
		emailOf: (memberID) => emails.get(memberID) ?? '',
		timeZoneOf: (memberID) => timeZones.get(memberID) || companyZone
	};
}
