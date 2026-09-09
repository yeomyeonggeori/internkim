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
	leaveDisplayRange
} from './supabase-leave-range';
import { annualLeaveTypeID } from './leave-policy-defaults';
import { leaveDaysInYear, leaveYearOf } from './leave-year-share';
import { leaveCountsAsUsage } from './supabase-leave-summary';
import {
	askTheRecord,
	employeeLeaveRequestOfRow,
	leaveInFull,
	leaveSpanAsked,
	supabaseLeavePreview,
	type LeaveRow
} from './supabase-leave';
import {
	companyDirectory,
	companySettings,
	everyLeaveBalance,
	leaveBalanceOfPerson,
	type RecordLeave
} from './attendance-record';
import { supabaseLeaveTypeDirectory, type LeaveTypeDirectory } from './supabase-leave-types';

type CompanyRow = { timezone: string };
type DirectoryMember = {
	id: string;
	email: string | null;
	name: string | null;
	timezone: string | null;
};

type MemberRow = DirectoryMember & { grantedDays: number | null };

type LeaveManagementSource = {
	company: CompanyRow;
	members: MemberRow[];
	leaves: LeaveRow[];
	targetYear: number;
	leaveTypes: LeaveTypeDirectory;
};

type LeaveManagementRows = { members: MemberRow[]; leaves: LeaveRow[] };

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
				)
		}
	};
}

export async function adjustSupabaseManagedLeave(input: LeaveManagementAdjustment): Promise<void> {
	const company = await readCompany();
	const answered = await leaveBalanceOfPerson(input.employeeEmail);
	const granted = answered.balances[0]?.grantedDays ?? null;
	if (granted === null) throw new EmployeeLeaveAPIError('invalidStatus', 409);
	const next = granted + input.amountMilliDays / 1000;
	if (next < 0) throw new EmployeeLeaveAPIError('insufficientBalance', 409);

	await askTheRecord('leave_grant_set', { personHint: input.employeeEmail, days: next });
}

export async function createSupabaseManagedPastLeave(input: LeaveManagementPastLeave): Promise<void> {
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
	const span = leaveSpanAsked(request, member.timezone || company.timezone);

	const filed = await askTheRecord<RecordLeave>('leave_request', {
		personHint: input.employeeEmail,
		kind: input.leaveTypeID,
		startsAt: span.startsAt,
		endsAt: span.endsAt,
		days: preview.totalDeductionMilliDays / 1000,
		note: input.reason
	});
	await askTheRecord('leave_decide', { leaveHint: filed.leaveID, decision: 'approved' });
}

export async function cancelSupabaseManagedLeaveRequest(requestID: string): Promise<void> {
	await askTheRecord('leave_delete', { leaveHint: requestID });
}

export async function correctSupabaseManagedLeaveTime(
	requestID: string,
	input: LeaveManagementTimeCorrection
): Promise<void> {
	const company = await readCompany();
	const member = await readMemberByEmail(input.employeeEmail);
	const timeZone = member.timezone || company.timezone;
	const existing = (await leaveInFull()).find((leave) => leave.id === requestID);
	if (!existing) throw new EmployeeLeaveAPIError('requestNotFound', 404);
	const localDate = companyDateOfTimestamp(existing.starts_at, timeZone);

	await askTheRecord('leave_update', {
		leaveHint: requestID,
		startsAt: companyDateTimeISO(localDate, input.startTime, timeZone),
		endsAt: companyDateTimeISO(localDate, input.endTime, timeZone)
	});
}

async function readLeaveManagementSource(): Promise<LeaveManagementSource> {
	const company = await readCompany();
	const rows = await leaveManagementRows();
	const leaveTypes = await supabaseLeaveTypeDirectory();
	const targetYear = leaveYearOf(
		companyDateOfTimestamp(new Date().toISOString(), company.timezone),
		leaveTypes.yearStart.month,
		leaveTypes.yearStart.day
	);
	return {
		company,
		members: rows.members,
		leaves: rows.leaves,
		targetYear,
		leaveTypes
	};
}

async function readCompany(): Promise<CompanyRow> {
	const settings = await companySettings();
	return { timezone: settings.timeZone };
}

async function leaveManagementMembers(): Promise<DirectoryMember[]> {
	const directory = await companyDirectory();
	return directory.people
		.map((person) => ({
			id: person.personID,
			email: person.email || null,
			name: person.name || null,
			timezone: person.timeZone || null
		}))
		.sort((left, right) => (left.email ?? '').localeCompare(right.email ?? ''));
}

async function leaveManagementRows(): Promise<LeaveManagementRows> {
	const [members, balances, leaves] = await Promise.all([
		leaveManagementMembers(),
		everyLeaveBalance(),
		leaveInFull()
	]);
	const grantedTo = new Map(
		balances.balances.map((balance) => [balance.personID, balance.grantedDays])
	);
	return {
		members: members.map((member) => ({ ...member, grantedDays: grantedTo.get(member.id) ?? null })),
		leaves
	};
}

async function readMemberByEmail(email: string): Promise<DirectoryMember> {
	const member = (await leaveManagementMembers()).find((row) => row.email === email);
	if (!member) throw new EmployeeLeaveAPIError('requestNotFound', 404);
	return member;
}

function trackingModeOf(source: LeaveManagementSource): EmployeeLeaveBalanceTrackingMode {
	return source.leaveTypes.trackingMode;
}

function employeeOf(
	member: MemberRow,
	source: LeaveManagementSource,
	trackingMode: EmployeeLeaveBalanceTrackingMode
): LeaveManagementEmployee {
	const granted = member.grantedDays;
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
		source.targetYear,
		source.leaveTypes.yearStart.month,
		source.leaveTypes.yearStart.day
	);
}
