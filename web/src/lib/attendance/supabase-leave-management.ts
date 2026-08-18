import { supabase } from '$lib/supabase';
import type {
	EmployeeLeaveRequest,
	EmployeeLeaveType,
	EmployeeLeaveUnit
} from '../../routes/attendance/leave/employee-leave-types';
import type {
	LeaveManagementAdjustment,
	LeaveManagementBalance,
	LeaveManagementDetail,
	LeaveManagementEmployee,
	LeaveManagementLedgerEntry,
	LeaveManagementPastLeave,
	LeaveManagementPayload,
	LeaveManagementTimeCorrection
} from '../../routes/attendance/management/leave-management-types';
import { leaveDisplayRange, leaveTimestampRange } from './supabase-leave-range';

type LeaveStatus = 'requested' | 'approved' | 'rejected';

type MemberRow = {
	id: string;
	name: string | null;
	email: string | null;
};

type LeaveRow = {
	id: string;
	member_id: string;
	kind: string;
	is_deducted: boolean;
	days: number;
	status: LeaveStatus;
	starts_at: string;
	ends_at: string;
	note: string | null;
	cancelled_at: string | null;
};

type LedgerRow = {
	id: string;
	operation_type: 'adjustment' | 'legal_correction';
	delta_days: number | null;
	effective_on: string;
	occurred_at: string;
	reason: string | null;
};

const leaveType: EmployeeLeaveType = {
	id: 'leave',
	name: '휴가',
	balanceMode: 'annual',
	allowedUnits: ['fullDay', 'halfDay', 'quarterDay'],
	includeInSummary: true,
	isActive: true,
	requiresHireDate: false
};

export async function supabaseLeaveManagement(employeeEmail = ''): Promise<LeaveManagementPayload> {
	const [members, timeZone, leave] = await Promise.all([visibleMembers(), companyTimeZone(), leaveRows()]);
	const years = await Promise.all(members.map(async (member) => [member.id, await memberCurrentYear(member.id)] as const));
	const yearByMemberID = new Map(years);
	const leaveByMemberID = groupByMemberID(leave);
	const remaining = await Promise.all(
		members.map(async (member) => [
			member.id,
			await memberLeaveRemaining(member.id, yearByMemberID.get(member.id) ?? new Date().getUTCFullYear())
		] as const)
	);
	const remainingByMemberID = new Map(remaining);
	const employees = members.map((member) =>
		employeeOf(
			member,
			leaveByMemberID.get(member.id) ?? [],
			yearByMemberID.get(member.id) ?? new Date().getUTCFullYear(),
			remainingByMemberID.get(member.id) ?? null,
			timeZone
		)
	);
	const detail = employeeEmail
		? await detailFor(employeeEmail, members, employees, leaveByMemberID, timeZone)
		: undefined;
	const managed = employees.some((employee) => employee.balances.length > 0);
	return {
		balanceTrackingMode: managed ? 'managed' : 'unlimited',
		leaveTypes: [{ ...leaveType, balanceMode: managed ? 'annual' : 'none' }],
		employees,
		detail
	};
}

export async function adjustSupabaseManagedLeave(input: LeaveManagementAdjustment): Promise<void> {
	const member = await memberByEmail(input.employeeEmail);
	const { error } = await supabase().rpc('admin_adjust_leave_balance', {
		target_member: member.id,
		delta_days: input.amountMilliDays / 1000,
		adjustment_reason: input.reason,
		adjustment_effective_on: input.effectiveOn,
		adjustment_expires_on: input.expiresOn || null
	});
	if (error) throw new Error(error.message);
}

export async function createSupabaseManagedPastLeave(input: LeaveManagementPastLeave): Promise<void> {
	const member = await memberByEmail(input.employeeEmail);
	const range = leaveTimestampRange(
		{
			leaveTypeID: input.leaveTypeID,
			unit: input.unit,
			startDate: input.startDate,
			endDate: input.endDate,
			partialPeriod: input.partialPeriod || undefined,
			startTime: input.startTime || undefined
		},
		await companyTimeZone()
	);
	const { error } = await supabase().rpc('admin_create_past_leave', {
		target_member: member.id,
		leave_days: daysFor(input.unit, input.startDate, input.endDate),
		leave_starts_at: range.startsAt,
		leave_ends_at: range.endsAt,
		leave_reason: input.reason
	});
	if (error) throw new Error(error.message);
}

export async function cancelSupabaseManagedLeave(requestID: string, employeeEmail: string): Promise<void> {
	const member = await memberByEmail(employeeEmail);
	await leaveForMember(requestID, member.id);
	const { error } = await supabase().rpc('admin_cancel_leave', {
		target_leave: requestID,
		cancel_reason: ''
	});
	if (error) throw new Error(error.message);
}

export async function correctSupabaseManagedLeaveTime(
	requestID: string,
	input: LeaveManagementTimeCorrection
): Promise<void> {
	const member = await memberByEmail(input.employeeEmail);
	const leave = await leaveForMember(requestID, member.id);
	const timeZone = await companyTimeZone();
	const date = leaveDisplayRange(leave.starts_at, leave.ends_at, leave.days, timeZone).startDate;
	const { error } = await supabase().rpc('admin_correct_leave_time', {
		target_leave: requestID,
		corrected_starts_at: dateTimeInZone(date, input.startTime, timeZone),
		corrected_ends_at: dateTimeInZone(date, input.endTime, timeZone),
		correction_reason: input.reason
	});
	if (error) throw new Error(error.message);
}

function employeeOf(
	member: MemberRow,
	leave: readonly LeaveRow[],
	targetYear: number,
	remainingDays: number | null,
	timeZone: string
): LeaveManagementEmployee {
	const active = leave.filter((row) => !row.cancelled_at && row.is_deducted && yearOf(row, timeZone) === targetYear);
	const usedMilliDays = milliDays(active.filter((row) => row.status === 'approved'));
	const reservedMilliDays = milliDays(active.filter((row) => row.status === 'requested'));
	const availableMilliDays = remainingDays === null ? 0 : Math.round(remainingDays * 1000) - reservedMilliDays;
	const grantedMilliDays = remainingDays === null ? 0 : availableMilliDays + reservedMilliDays + usedMilliDays;
	const balance: LeaveManagementBalance = {
		leaveTypeID: leaveType.id,
		leaveTypeName: leaveType.name,
		grantedMilliDays,
		availableMilliDays,
		reservedMilliDays,
		usedMilliDays,
		expiredMilliDays: 0,
		nextExpiryMilliDays: 0
	};
	return {
		email: member.email ?? '',
		displayName: member.name || (member.email ?? '').split('@')[0],
		grantedMilliDays,
		availableMilliDays,
		reservedMilliDays,
		usedMilliDays,
		expiringMilliDays: 0,
		balances: remainingDays === null ? [] : [balance]
	};
}

async function detailFor(
	employeeEmail: string,
	members: readonly MemberRow[],
	employees: readonly LeaveManagementEmployee[],
	leaveByMemberID: ReadonlyMap<string, LeaveRow[]>,
	timeZone: string
): Promise<LeaveManagementDetail> {
	const member = memberFromVisibleMembers(employeeEmail, members);
	const employee = employees.find((candidate) => candidate.email === employeeEmail);
	if (!employee) throw new Error('employee is not visible in this company');
	const ledger = await supabase()
		.from('leave_ledger_entry')
		.select('id, operation_type, delta_days, effective_on, occurred_at, reason')
		.eq('member_id', member.id)
		.order('occurred_at', { ascending: false })
		.returns<LedgerRow[]>();
	if (ledger.error) throw new Error(ledger.error.message);
	return {
		employee,
		requests: (leaveByMemberID.get(member.id) ?? []).map((row) => requestOf(row, timeZone)),
		ledgerEntries: ledger.data.map((entry) => ledgerOf(entry, employee.availableMilliDays))
	};
}

function requestOf(row: LeaveRow, timeZone: string): EmployeeLeaveRequest {
	const cancelled = Boolean(row.cancelled_at);
	const range = leaveDisplayRange(row.starts_at, row.ends_at, row.days, timeZone);
	return {
		id: row.id,
		leaveTypeID: leaveType.id,
		leaveTypeName: leaveType.name,
		status: cancelled ? 'cancelled' : row.status === 'requested' ? 'pending' : row.status,
		unit: unitOf(row.days),
		...range,
		deductionMilliDays: Math.round(row.days * 1000),
		reason: row.note ?? '',
		attachments: [],
		canCancel: !cancelled && row.status === 'requested',
		canEdit: false,
		canResubmit: false,
		revision: 0,
		createdAt: row.starts_at,
		updatedAt: row.starts_at
	};
}

function ledgerOf(entry: LedgerRow, balanceAfterMilliDays: number): LeaveManagementLedgerEntry {
	return {
		id: entry.id,
		operationType: entry.operation_type === 'legal_correction' ? 'legalCorrection' : 'adjustment',
		leaveTypeID: leaveType.id,
		leaveTypeName: leaveType.name,
		deltaMilliDays: Math.round((entry.delta_days ?? 0) * 1000),
		balanceAfterMilliDays,
		effectiveOn: entry.effective_on,
		occurredAt: entry.occurred_at,
		reason: entry.reason ?? undefined
	};
}

async function visibleMembers(): Promise<MemberRow[]> {
	const members = await supabase()
		.from('member')
		.select('id, name, email')
		.neq('status', 'withdrawn')
		.returns<MemberRow[]>();
	if (members.error) throw new Error(members.error.message);
	return members.data.filter((member) => Boolean(member.email));
}

async function memberByEmail(email: string): Promise<MemberRow> {
	return memberFromVisibleMembers(email, await visibleMembers());
}

function memberFromVisibleMembers(email: string, members: readonly MemberRow[]): MemberRow {
	const member = members.find((candidate) => candidate.email === email);
	if (!member) throw new Error('employee is not visible in this company');
	return member;
}

async function leaveRows(): Promise<LeaveRow[]> {
	const leave = await supabase()
		.from('leave')
		.select('id, member_id, kind, is_deducted, days, status, starts_at, ends_at, note, cancelled_at')
		.order('starts_at', { ascending: false })
		.returns<LeaveRow[]>();
	if (leave.error) throw new Error(leave.error.message);
	return leave.data;
}

async function leaveForMember(requestID: string, memberID: string): Promise<LeaveRow> {
	const leave = await supabase()
		.from('leave')
		.select('id, member_id, kind, is_deducted, days, status, starts_at, ends_at, note, cancelled_at')
		.eq('id', requestID)
		.eq('member_id', memberID)
		.single<LeaveRow>();
	if (leave.error) throw new Error(leave.error.message);
	return leave.data;
}

async function companyTimeZone(): Promise<string> {
	const company = await supabase().from('company').select('timezone').limit(1).single<{ timezone: string }>();
	if (company.error) throw new Error(company.error.message);
	return company.data.timezone;
}

async function memberCurrentYear(memberID: string): Promise<number> {
	const today = await supabase().rpc('member_today', { target_member: memberID });
	if (today.error || typeof today.data !== 'string') throw new Error(today.error?.message ?? 'member_today returned an invalid date');
	const year = Number(today.data.slice(0, 4));
	if (!Number.isInteger(year)) throw new Error('member_today returned an invalid date');
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
	if (!Number.isFinite(days)) throw new Error('member_leave_remaining returned an invalid balance');
	return days;
}

function groupByMemberID(rows: readonly LeaveRow[]): Map<string, LeaveRow[]> {
	const grouped = new Map<string, LeaveRow[]>();
	for (const row of rows) grouped.set(row.member_id, [...(grouped.get(row.member_id) ?? []), row]);
	return grouped;
}

function yearOf(row: LeaveRow, timeZone: string): number {
	return Number(leaveDisplayRange(row.starts_at, row.ends_at, row.days, timeZone).startDate.slice(0, 4));
}

function milliDays(rows: readonly LeaveRow[]): number {
	return rows.reduce((total, row) => total + Math.round(row.days * 1000), 0);
}

function unitOf(days: number): EmployeeLeaveUnit {
	if (days <= 0.25) return 'quarterDay';
	if (days <= 0.5) return 'halfDay';
	return 'fullDay';
}

function daysFor(unit: EmployeeLeaveUnit, startDate: string, endDate: string): number {
	if (unit === 'halfDay') return 0.5;
	if (unit === 'quarterDay') return 0.25;
	const start = new Date(`${startDate}T00:00:00Z`);
	const end = new Date(`${endDate}T00:00:00Z`);
	return Math.floor((end.getTime() - start.getTime()) / 86400000) + 1;
}

function dateTimeInZone(date: string, time: string, timeZone: string): string {
	return leaveTimestampRange(
		{ leaveTypeID: leaveType.id, unit: 'quarterDay', startDate: date, partialPeriod: 'custom', startTime: time },
		timeZone
	).startsAt;
}
