import { supabase } from '../supabase';
import type { EmployeeLeaveUnit } from '../../routes/attendance/leave/employee-leave-types';
import type {
	LeaveManagementAdjustment,
	LeaveManagementDetail,
	LeaveManagementEmployee,
	LeaveManagementPastLeave,
	LeaveManagementPayload,
	LeaveManagementTimeCorrection
} from '../../routes/attendance/management/leave-management-types';
import {
	centralLeaveType,
	leaveManagementEmployee,
	leaveManagementLedgerEntry,
	leaveManagementMemberTimeZone,
	leaveManagementRequest,
	leaveRowsByMemberID,
	type SupabaseLeaveManagementLedgerRow,
	type SupabaseLeaveManagementMember,
	type SupabaseLeaveManagementRow
} from './supabase-leave-management-model';
import { leaveDisplayRange, leaveTimestampRange } from './supabase-leave-range';

export async function supabaseLeaveManagement(employeeEmail = ''): Promise<LeaveManagementPayload> {
	const [members, companyZone, leave] = await Promise.all([visibleMembers(), companyTimeZone(), leaveRows()]);
	const currentDates = await Promise.all(
		members.map(async (member) => [member.id, await memberToday(member.id)] as const)
	);
	const currentDateByMemberID = new Map(currentDates);
	const leaveByMemberID = leaveRowsByMemberID(leave);
	const remaining = await Promise.all(
		members.map(async (member) => {
			const currentDate = currentDateByMemberID.get(member.id);
			if (!currentDate) throw new Error('member_today did not return a date for a visible member');
			return [member.id, await memberLeaveRemaining(member.id, Number(currentDate.slice(0, 4)))] as const;
		})
	);
	const remainingByMemberID = new Map(remaining);
	const employees = members.map((member) => {
		const currentDate = currentDateByMemberID.get(member.id);
		if (!currentDate) throw new Error('member_today did not return a date for a visible member');
		return leaveManagementEmployee(
			member,
			leaveByMemberID.get(member.id) ?? [],
			currentDate,
			remainingByMemberID.get(member.id) ?? null,
			leaveManagementMemberTimeZone(member, companyZone)
		);
	});
	const detail = employeeEmail
		? await detailFor(employeeEmail, members, employees, leaveByMemberID, companyZone)
		: undefined;
	const managed = employees.some((employee) => employee.balances.length > 0);
	return {
		balanceTrackingMode: managed ? 'managed' : 'unlimited',
		leaveTypes: [{ ...centralLeaveType, balanceMode: managed ? 'annual' : 'none' }],
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
	const timeZone = leaveManagementMemberTimeZone(member, await companyTimeZone());
	const range = leaveTimestampRange(
		{
			leaveTypeID: input.leaveTypeID,
			unit: input.unit,
			startDate: input.startDate,
			endDate: input.endDate,
			partialPeriod: input.partialPeriod || undefined,
			startTime: input.startTime || undefined
		},
		timeZone
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
	const timeZone = leaveManagementMemberTimeZone(member, await companyTimeZone());
	const date = leaveDisplayRange(leave.starts_at, leave.ends_at, leave.days, timeZone).startDate;
	const { error } = await supabase().rpc('admin_correct_leave_time', {
		target_leave: requestID,
		corrected_starts_at: dateTimeInZone(date, input.startTime, timeZone),
		corrected_ends_at: dateTimeInZone(date, input.endTime, timeZone),
		correction_reason: input.reason
	});
	if (error) throw new Error(error.message);
}

async function detailFor(
	employeeEmail: string,
	members: readonly SupabaseLeaveManagementMember[],
	employees: readonly LeaveManagementEmployee[],
	leaveByMemberID: ReadonlyMap<string, SupabaseLeaveManagementRow[]>,
	companyZone: string
): Promise<LeaveManagementDetail> {
	const member = memberFromVisibleMembers(employeeEmail, members);
	const timeZone = leaveManagementMemberTimeZone(member, companyZone);
	const employee = employees.find((candidate) => candidate.email === employeeEmail);
	if (!employee) throw new Error('employee is not visible in this company');
	const ledger = await supabase()
		.from('leave_ledger_entry')
		.select('id, operation_type, delta_days, effective_on, occurred_at, reason')
		.eq('member_id', member.id)
		.order('occurred_at', { ascending: false })
		.returns<SupabaseLeaveManagementLedgerRow[]>();
	if (ledger.error) throw new Error(ledger.error.message);
	return {
		employee,
		requests: (leaveByMemberID.get(member.id) ?? []).map((row) => leaveManagementRequest(row, timeZone)),
		ledgerEntries: ledger.data.map(leaveManagementLedgerEntry)
	};
}

async function visibleMembers(): Promise<SupabaseLeaveManagementMember[]> {
	const members = await supabase()
		.from('member')
		.select('id, name, email, timezone')
		.neq('status', 'withdrawn')
		.returns<SupabaseLeaveManagementMember[]>();
	if (members.error) throw new Error(members.error.message);
	return members.data.filter((member) => Boolean(member.email));
}

async function memberByEmail(email: string): Promise<SupabaseLeaveManagementMember> {
	return memberFromVisibleMembers(email, await visibleMembers());
}

function memberFromVisibleMembers(
	email: string,
	members: readonly SupabaseLeaveManagementMember[]
): SupabaseLeaveManagementMember {
	const member = members.find((candidate) => candidate.email === email);
	if (!member) throw new Error('employee is not visible in this company');
	return member;
}

async function leaveRows(): Promise<SupabaseLeaveManagementRow[]> {
	const leave = await supabase()
		.from('leave')
		.select('id, member_id, kind, is_deducted, days, status, starts_at, ends_at, note, cancelled_at')
		.order('starts_at', { ascending: false })
		.returns<SupabaseLeaveManagementRow[]>();
	if (leave.error) throw new Error(leave.error.message);
	return leave.data;
}

async function leaveForMember(requestID: string, memberID: string): Promise<SupabaseLeaveManagementRow> {
	const leave = await supabase()
		.from('leave')
		.select('id, member_id, kind, is_deducted, days, status, starts_at, ends_at, note, cancelled_at')
		.eq('id', requestID)
		.eq('member_id', memberID)
		.single<SupabaseLeaveManagementRow>();
	if (leave.error) throw new Error(leave.error.message);
	return leave.data;
}

async function companyTimeZone(): Promise<string> {
	const company = await supabase().from('company').select('timezone').limit(1).single<{ timezone: string }>();
	if (company.error) throw new Error(company.error.message);
	return company.data.timezone;
}

async function memberToday(memberID: string): Promise<string> {
	const today = await supabase().rpc('member_today', { target_member: memberID });
	if (today.error || typeof today.data !== 'string') throw new Error(today.error?.message ?? 'member_today returned an invalid date');
	if (!/^\d{4}-\d{2}-\d{2}$/.test(today.data)) throw new Error('member_today returned an invalid date');
	return today.data;
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


function daysFor(unit: EmployeeLeaveUnit, startDate: string, endDate: string): number {
	if (unit === 'halfDay') return 0.5;
	if (unit === 'quarterDay') return 0.25;
	const start = new Date(`${startDate}T00:00:00Z`);
	const end = new Date(`${endDate}T00:00:00Z`);
	return Math.floor((end.getTime() - start.getTime()) / 86400000) + 1;
}

function dateTimeInZone(date: string, time: string, timeZone: string): string {
	return leaveTimestampRange(
		{ leaveTypeID: centralLeaveType.id, unit: 'quarterDay', startDate: date, partialPeriod: 'custom', startTime: time },
		timeZone
	).startsAt;
}
