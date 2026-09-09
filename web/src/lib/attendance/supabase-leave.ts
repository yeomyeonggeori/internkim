import { announceToTheCompany } from './announce-attendance';
import { invokeTool, ToolRefused } from '$lib/public-api-call';
import {
	companyDirectory,
	companySettings,
	everyLeaveOfTheCompany,
	myLeaveBalance,
	timeZoneOfPerson,
	type RecordLeave
} from './attendance-record';
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

export function leaveRowOf(taken: RecordLeave): LeaveRow {
	return {
		id: taken.leaveID,
		member_id: taken.personID,
		kind: taken.kindID,
		is_paid: taken.isPaid,
		is_deducted: taken.isDeducted,
		days: taken.days,
		status: taken.status,
		starts_at: taken.startsAt,
		ends_at: taken.endsAt,
		note: taken.note
	};
}

export async function leaveInFull(): Promise<LeaveRow[]> {
	const answered = await everyLeaveOfTheCompany();
	return answered.leave.map(leaveRowOf);
}

export async function supabaseEmployeeLeave(): Promise<EmployeeLeavePayload> {
	const directory = await supabaseLeaveTypeDirectory();
	const answeredBalance = await myLeaveBalance();
	const balanceOfMine = answeredBalance.balances[0];
	if (!balanceOfMine) throw new Error('the record answered no balance for the requester');
	const memberID = balanceOfMine.personID;
	const rows = (await leaveInFull())
		.filter((row) => row.member_id === memberID)
		.sort((left, right) => right.starts_at.localeCompare(left.starts_at));
	const timeZone = await memberTimeZone(memberID);
	const mappedLeave = rows.map((row) => ({
		row,
		request: employeeLeaveRequestOfRow(row, timeZone, directory.nameOf)
	}));
	const requests = mappedLeave.map(({ request }) => request);
	const targetYear = answeredBalance.year;
	const remainingDays = balanceOfMine.remainingDays;
	const balance = summarizeSupabaseLeave(
		mappedLeave.map(({ row, request }) => ({
			days: row.days,
			status: row.status,
			isDeducted: row.is_deducted,
			localStartDate: request.startDate,
			localEndDate: request.endDate || request.startDate
		})),
		targetYear,
		remainingDays,
		directory.yearStart.month,
		directory.yearStart.day
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
export function leaveSpanAsked(
	request: EmployeeLeavePreviewRequest,
	timeZone: string
): { startsAt: string; endsAt: string } {
	if (request.unit === 'fullDay') {
		return { startsAt: request.startDate, endsAt: request.endDate || request.startDate };
	}
	return leaveTimestampRange(request, timeZone);
}

export async function createSupabaseLeaveRequest(request: EmployeeLeaveSubmission): Promise<void> {
	const preview = await supabaseLeavePreview(request);
	const span = leaveSpanAsked(request, await companyTimeZone());
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
	const decided = await askTheRecord<RecordLeave>('leave_decide', {
		leaveHint: requestID,
		decision: decision.action === 'approve' ? 'approved' : 'rejected'
	});
	return approvalOf(leaveRowOf(decided), await memberDirectory(), await supabaseLeaveTypeDirectory());
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
	const companyZone = await companyTimeZone();
	const directory = await companyDirectory();
	return timeZoneOfPerson(
		directory.people.find((person) => person.personID === memberID),
		companyZone
	);
}

async function companyTimeZone(): Promise<string> {
	return (await companySettings()).timeZone;
}

async function memberDirectory(): Promise<MemberDirectory> {
	const companyZone = await companyTimeZone();
	const directory = await companyDirectory();
	const emails = new Map(directory.people.map((person) => [person.personID, person.email]));
	const timeZones = new Map(
		directory.people.map((person) => [person.personID, timeZoneOfPerson(person, companyZone)])
	);
	return {
		emailOf: (memberID) => emails.get(memberID) ?? '',
		timeZoneOf: (memberID) => timeZones.get(memberID) || companyZone
	};
}

// The record refuses through the public API, and the leave screens speak one
// error type. A refusal keeps its status so the screen can tell a leave that
// may no longer be touched from one the caller may not touch at all.
export async function askTheRecord<Answer>(
	name: string,
	input: Record<string, unknown>
): Promise<Answer> {
	try {
		return await invokeTool<Answer>(name, input);
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
