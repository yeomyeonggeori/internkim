import { supabase } from '$lib/supabase';
import type {
	EmployeeLeaveBalanceTrackingMode,
	EmployeeLeavePreviewRequest,
} from '../../routes/attendance/leave/employee-leave-types';
import type {
	LeaveManagementAdjustment,
	LeaveManagementEmployee,
	LeaveManagementPastLeave,
	LeaveManagementPayload,
	LeaveManagementTimeCorrection
} from '../../routes/attendance/management/leave-management-types';
import { EmployeeLeaveAPIError } from '../../routes/attendance/leave/employee-leave-api-error';
import {
	companyDateOfTimestamp,
	companyDateTimeISO,
	leaveDisplayRange,
	leaveTimestampRange
} from './supabase-leave-range';
import { annualLeaveTypeID } from './leave-policy-defaults';
import { leaveDaysInYear } from './leave-year-share';
import { leaveCountsAsUsage } from './supabase-leave-summary';
import {
	employeeLeaveRequestOfRow,
	supabaseLeavePreview,
	type LeaveRow
} from './supabase-leave';
import { supabaseLeaveTypeDirectory, type LeaveTypeDirectory } from './supabase-leave-types';

type CompanyRow = { timezone: string; leave_days: number | null };
type MemberRow = {
	id: string;
	email: string | null;
	name: string | null;
	leave_days: number | null;
	timezone: string | null;
};

type LeaveManagementSource = {
	company: CompanyRow;
	members: MemberRow[];
	leaves: LeaveRow[];
	targetYear: number;
	leaveTypes: LeaveTypeDirectory;
};

const leaveColumns = 'id, member_id, kind, is_paid, is_deducted, days, status, starts_at, ends_at, note';
const memberColumns = 'id, email, name, leave_days, timezone';

function timeZoneOf(member: MemberRow, source: LeaveManagementSource): string {
	return member.timezone || source.company.timezone;
}

export async function supabaseLeaveManagement(employeeEmail = ''): Promise<LeaveManagementPayload> {
	const source = await readLeaveManagementSource();
	const trackingMode = trackingModeOf(source);
	const employees = source.members.map((member) => employeeOf(member, source, trackingMode));
	const payload: LeaveManagementPayload = {
		balanceTrackingMode: trackingMode,
		leaveTypes: source.leaveTypes.offered,
		employees
	};

	const selected = source.members.find((member) => member.email === employeeEmail);
	if (!employeeEmail || !selected) return payload;
	return {
		...payload,
		detail: {
			employee: employeeOf(selected, source, trackingMode),
			requests: source.leaves
				.filter((leave) => leave.member_id === selected.id)
				.sort((left, right) => right.starts_at.localeCompare(left.starts_at))
				.map((leave) =>
					employeeLeaveRequestOfRow(leave, timeZoneOf(selected, source), source.leaveTypes.nameOf)
				),
			ledgerEntries: []
		}
	};
}

export async function adjustSupabaseManagedLeave(input: LeaveManagementAdjustment): Promise<void> {
	const company = await readCompany();
	const member = await readMemberByEmail(input.employeeEmail);
	const granted = member.leave_days ?? company.leave_days;
	if (granted === null) throw new EmployeeLeaveAPIError('invalidStatus', 409);
	const next = granted + input.amountMilliDays / 1000;
	if (next < 0) throw new EmployeeLeaveAPIError('insufficientBalance', 409);

	const saved = await supabase().rpc('member_leave_days_set', {
		target_member: member.id,
		granted_days: next
	});
	if (saved.error) throw new EmployeeLeaveAPIError(null, 403);
}

export async function createSupabaseManagedPastLeave(input: LeaveManagementPastLeave): Promise<void> {
	const directory = await supabaseLeaveTypeDirectory();
	const company = await readCompany();
	const member = await readMemberByEmail(input.employeeEmail);
	const request: EmployeeLeavePreviewRequest = {
		leaveTypeID: input.leaveTypeID,
		unit: input.unit,
		startDate: input.startDate,
		endDate: input.endDate || input.startDate,
		partialPeriod: input.partialPeriod || undefined,
		startTime: input.startTime || undefined
	};
	const preview = await supabaseLeavePreview(request);
	const range = leaveTimestampRange(request, member.timezone || company.timezone);

	const recorded = await supabase()
		.from('leave')
		.insert({
			member_id: member.id,
			kind: input.leaveTypeID,
			is_paid: directory.isPaid(input.leaveTypeID),
			is_deducted: directory.deductsAnnualBalance(input.leaveTypeID),
			days: preview.totalDeductionMilliDays / 1000,
			status: 'approved',
			starts_at: range.startsAt,
			ends_at: range.endsAt,
			note: input.reason
		})
		.select('id')
		.returns<{ id: string }[]>();
	if (recorded.error) throw new Error(recorded.error.message);
	if (recorded.data.length === 0) throw new EmployeeLeaveAPIError(null, 403);
}

export async function cancelSupabaseManagedLeaveRequest(requestID: string): Promise<void> {
	const withdrawn = await supabase()
		.from('leave')
		.delete()
		.eq('id', requestID)
		.select('id')
		.returns<{ id: string }[]>();
	if (withdrawn.error) throw new Error(withdrawn.error.message);
	if (withdrawn.data.length === 0) throw new EmployeeLeaveAPIError('requestNotFound', 404);
}

export async function correctSupabaseManagedLeaveTime(
	requestID: string,
	input: LeaveManagementTimeCorrection
): Promise<void> {
	const company = await readCompany();
	const member = await readMemberByEmail(input.employeeEmail);
	const timeZone = member.timezone || company.timezone;
	const existing = await supabase()
		.from('leave')
		.select(leaveColumns)
		.eq('id', requestID)
		.single<LeaveRow>();
	if (existing.error) throw new EmployeeLeaveAPIError('requestNotFound', 404);
	const localDate = companyDateOfTimestamp(existing.data.starts_at, timeZone);

	const corrected = await supabase()
		.from('leave')
		.update({
			starts_at: companyDateTimeISO(localDate, input.startTime, timeZone),
			ends_at: companyDateTimeISO(localDate, input.endTime, timeZone)
		})
		.eq('id', requestID)
		.select('id')
		.returns<{ id: string }[]>();
	if (corrected.error) throw new Error(corrected.error.message);
	if (corrected.data.length === 0) throw new EmployeeLeaveAPIError(null, 403);
}

async function readLeaveManagementSource(): Promise<LeaveManagementSource> {
	const company = await readCompany();
	const members = await supabase()
		.from('member')
		.select(memberColumns)
		.order('email')
		.returns<MemberRow[]>();
	if (members.error) throw new Error(members.error.message);
	const leaves = await supabase().from('leave').select(leaveColumns).returns<LeaveRow[]>();
	if (leaves.error) throw new Error(leaves.error.message);
	const targetYear = Number(
		companyDateOfTimestamp(new Date().toISOString(), company.timezone).slice(0, 4)
	);
	return {
		company,
		members: members.data,
		leaves: leaves.data,
		targetYear,
		leaveTypes: await supabaseLeaveTypeDirectory()
	};
}

async function readCompany(): Promise<CompanyRow> {
	const company = await supabase()
		.from('company')
		.select('timezone, leave_days')
		.limit(1)
		.single<CompanyRow>();
	if (company.error) throw new Error(company.error.message);
	return company.data;
}

async function readMemberByEmail(email: string): Promise<MemberRow> {
	const member = await supabase()
		.from('member')
		.select(memberColumns)
		.eq('email', email)
		.single<MemberRow>();
	if (member.error) throw new EmployeeLeaveAPIError('requestNotFound', 404);
	return member.data;
}

function trackingModeOf(source: LeaveManagementSource): EmployeeLeaveBalanceTrackingMode {
	return source.company.leave_days === null ? 'unlimited' : 'managed';
}

function employeeOf(
	member: MemberRow,
	source: LeaveManagementSource,
	trackingMode: EmployeeLeaveBalanceTrackingMode
): LeaveManagementEmployee {
	const granted = member.leave_days ?? source.company.leave_days;
	const grantedMilliDays =
		trackingMode === 'unlimited' || granted === null ? 0 : Math.round(granted * 1000);

	let usedMilliDays = 0;
	let reservedMilliDays = 0;
	for (const leave of source.leaves) {
		if (leave.member_id !== member.id) continue;
		if (!leaveCountsAsUsage(trackingMode, leave.is_deducted)) continue;
		const days = daysFallingInTargetYear(leave, source, timeZoneOf(member, source));
		if (days === 0) continue;
		const milliDays = Math.round(days * 1000);
		if (leave.status === 'approved') usedMilliDays += milliDays;
		if (leave.status === 'requested') reservedMilliDays += milliDays;
	}

	const availableMilliDays = trackingMode === 'unlimited' ? 0 : grantedMilliDays - usedMilliDays;
	return {
		email: member.email ?? '',
		displayName: member.name ?? member.email ?? '',
		grantedMilliDays,
		availableMilliDays,
		reservedMilliDays,
		usedMilliDays,
		expiringMilliDays: 0,
		balances: [
			{
				leaveTypeID: annualLeaveTypeID,
				leaveTypeName: source.leaveTypes.nameOf(annualLeaveTypeID),
				grantedMilliDays,
				availableMilliDays,
				reservedMilliDays,
				usedMilliDays,
				expiredMilliDays: 0,
				nextExpiryMilliDays: 0
			}
		]
	};
}

function daysFallingInTargetYear(
	leave: LeaveRow,
	source: LeaveManagementSource,
	timeZone: string
): number {
	const range = leaveDisplayRange(leave.starts_at, leave.ends_at, leave.days, timeZone);
	return leaveDaysInYear(
		leave.days,
		range.startDate,
		range.endDate || range.startDate,
		source.targetYear
	);
}
