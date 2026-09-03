import { announceToTheCompany } from './announce-attendance';
import { invokeTool, ToolRefused } from '$lib/public-api-call';
import { supabase } from '$lib/supabase';
import type {
	EmployeeLeaveErrorCode,
	EmployeeLeavePayload,
	EmployeeLeavePreview,
	EmployeeLeavePreviewRequest,
	EmployeeLeaveRequest,
	EmployeeLeaveStatus,
	EmployeeLeaveSubmission,
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
import {
	supabaseLeaveTypeDirectory,
	withAnnualBalance,
	type LeaveTypeDirectory
} from './supabase-leave-types';

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

async function leaveInFull(): Promise<LeaveRow[]> {
	const rows = await supabase().rpc('leave_in_full');
	if (rows.error) throw new Error(rows.error.message);
	return (rows.data ?? []) as LeaveRow[];
}

export async function supabaseEmployeeLeave(): Promise<EmployeeLeavePayload> {
	const directory = await supabaseLeaveTypeDirectory();
	const memberID = await myMemberID();
	const rows = (await leaveInFull())
		.filter((row) => row.member_id === memberID)
		.sort((left, right) => right.starts_at.localeCompare(left.starts_at));
	const timeZone = await memberTimeZone(memberID);
	const mappedLeave = rows.map((row) => ({
		row,
		request: employeeLeaveRequestOfRow(row, timeZone, directory.nameOf)
	}));
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
	return {
		balanceTrackingMode: balance.trackingMode,
		leaveTypes: withAnnualBalance(directory.offered, directory, balance.summary),
		summary: balance.summary,
		requests
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

// A whole day is named by its dates and the record decides the moments it
// covers; a part of a day is the moments themselves, because no date can say
// which quarter of the day was taken.
export async function leaveSpanAsked(
	request: EmployeeLeavePreviewRequest
): Promise<{ startsAt: string; endsAt: string }> {
	if (request.unit === 'fullDay') {
		return { startsAt: request.startDate, endsAt: request.endDate || request.startDate };
	}
	return leaveTimestampRange(request, await companyTimeZone());
}

export async function createSupabaseLeaveRequest(request: EmployeeLeaveSubmission): Promise<void> {
	const preview = await supabaseLeavePreview(request);
	const span = await leaveSpanAsked(request);
	await askTheRecord('leave_request', {
		kind: request.leaveTypeID,
		startsAt: span.startsAt,
		endsAt: span.endsAt,
		days: preview.totalDeductionMilliDays / 1000,
		note: request.reason
	});
	void announceToTheCompany('leave');
}

export async function cancelSupabaseLeaveRequest(requestID: string): Promise<void> {
	await askTheRecord('leave_delete', { leaveHint: requestID });
}

export async function supabaseLeaveApprovalInbox(): Promise<LeaveApprovalInbox> {
	const directory = await supabaseLeaveTypeDirectory();
	const rows = (await leaveInFull())
		.filter((row) => row.status === 'requested')
		.sort((left, right) => left.starts_at.localeCompare(right.starts_at));

	const members = await memberDirectory();
	const pending = rows.map((row) => approvalOf(row, members, directory));
	return { pendingCount: pending.length, pending };
}

export async function decideSupabaseLeave(
	requestID: string,
	decision: LeaveApprovalDecision
): Promise<LeaveApprovalRequest> {
	await askTheRecord('leave_decide', {
		leaveHint: requestID,
		decision: decision.action === 'approve' ? 'approved' : 'rejected'
	});
	const row = (await leaveInFull()).find((each) => each.id === requestID);
	if (!row) throw new EmployeeLeaveAPIError('requestNotFound', 404);
	return approvalOf(row, await memberDirectory(), await supabaseLeaveTypeDirectory());
}

export function employeeLeaveRequestOfRow(
	row: LeaveRow,
	timeZone: string,
	nameOf: (leaveTypeID: string) => string
): EmployeeLeaveRequest {
	const status = statusWords[row.status];
	const range = leaveDisplayRange(row.starts_at, row.ends_at, row.days, timeZone);
	return {
		id: row.id,
		leaveTypeID: row.kind,
		leaveTypeName: nameOf(row.kind),
		status,
		unit: unitOf(row.days),
		...range,
		deductionMilliDays: Math.round(row.days * 1000),
		reason: row.note ?? '',
		canCancel: status === 'pending',
		createdAt: row.starts_at
	};
}

function approvalOf(
	row: LeaveRow,
	members: MemberDirectory,
	directory: LeaveTypeDirectory
): LeaveApprovalRequest {
	const timeZone = members.timeZoneOf(row.member_id);
	return {
		id: row.id,
		employeeEmail: members.emailOf(row.member_id),
		leaveTypeID: row.kind,
		leaveTypeName: directory.nameOf(row.kind),
		balanceMode: directory.ownsAnnualBalance(row.kind) ? 'annual' : 'none',
		status: statusWords[row.status],
		unit: unitOf(row.days),
		...leaveDisplayRange(row.starts_at, row.ends_at, row.days, timeZone),
		deductionMilliDays: Math.round(row.days * 1000),
		reason: row.note ?? '',
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

async function memberTimeZone(memberID: string): Promise<string> {
	const member = await supabase()
		.from('member')
		.select('timezone')
		.eq('id', memberID)
		.single<{ timezone: string | null }>();
	if (member.error) throw new Error(member.error.message);
	return member.data.timezone || (await companyTimeZone());
}

async function companyTimeZone(): Promise<string> {
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

// The record refuses through the public API, and the leave screens speak one
// error type. A refusal keeps its status so the screen can tell a leave that
// may no longer be touched from one the caller may not touch at all.
async function askTheRecord(name: string, input: Record<string, unknown>): Promise<void> {
	try {
		await invokeTool(name, input);
	} catch (refusal) {
		if (refusal instanceof ToolRefused) {
			throw new EmployeeLeaveAPIError(refusalCode(refusal.status), refusal.status);
		}
		throw refusal;
	}
}

function refusalCode(status: number): EmployeeLeaveErrorCode | null {
	if (status === 404) return 'requestNotFound';
	if (status === 403 || status === 409) return 'invalidStatus';
	if (status === 400) return 'invalidInput';
	return null;
}
