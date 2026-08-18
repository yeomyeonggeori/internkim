import type {
	EmployeeLeaveRequest,
	EmployeeLeaveType,
	EmployeeLeaveUnit
} from '../../routes/attendance/leave/employee-leave-types';
import type {
	LeaveManagementBalance,
	LeaveManagementEmployee,
	LeaveManagementLedgerEntry
} from '../../routes/attendance/management/leave-management-types';
import { leaveDisplayRange } from './supabase-leave-range';

export type SupabaseLeaveManagementMember = {
	id: string;
	name: string | null;
	email: string | null;
};

export type SupabaseLeaveManagementRow = {
	id: string;
	member_id: string;
	kind: string;
	is_deducted: boolean;
	days: number;
	status: 'requested' | 'approved' | 'rejected';
	starts_at: string;
	ends_at: string;
	note: string | null;
	cancelled_at: string | null;
};

export type SupabaseLeaveManagementLedgerRow = {
	id: string;
	operation_type: 'adjustment' | 'legal_correction';
	delta_days: number | null;
	effective_on: string;
	occurred_at: string;
	reason: string | null;
};

export const centralLeaveType: EmployeeLeaveType = {
	id: 'leave',
	name: '휴가',
	balanceMode: 'annual',
	allowedUnits: ['fullDay', 'halfDay', 'quarterDay'],
	includeInSummary: true,
	isActive: true,
	requiresHireDate: false
};

export function leaveManagementEmployee(
	member: SupabaseLeaveManagementMember,
	leave: readonly SupabaseLeaveManagementRow[],
	targetYear: number,
	remainingDays: number | null,
	timeZone: string
): LeaveManagementEmployee {
	const active = leave.filter(
		(row) => !row.cancelled_at && row.is_deducted && yearOf(row, timeZone) === targetYear
	);
	const usedMilliDays = milliDays(active.filter((row) => row.status === 'approved'));
	const reservedMilliDays = milliDays(active.filter((row) => row.status === 'requested'));
	const availableMilliDays = remainingDays === null ? 0 : Math.round(remainingDays * 1000) - reservedMilliDays;
	const grantedMilliDays = remainingDays === null ? 0 : availableMilliDays + reservedMilliDays + usedMilliDays;
	const balance: LeaveManagementBalance = {
		leaveTypeID: centralLeaveType.id,
		leaveTypeName: centralLeaveType.name,
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

export function leaveManagementRequest(
	row: SupabaseLeaveManagementRow,
	timeZone: string
): EmployeeLeaveRequest {
	const cancelled = Boolean(row.cancelled_at);
	const range = leaveDisplayRange(row.starts_at, row.ends_at, row.days, timeZone);
	return {
		id: row.id,
		leaveTypeID: centralLeaveType.id,
		leaveTypeName: centralLeaveType.name,
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

export function leaveManagementLedgerEntry(
	entry: SupabaseLeaveManagementLedgerRow
): LeaveManagementLedgerEntry {
	return {
		id: entry.id,
		operationType: entry.operation_type === 'legal_correction' ? 'legalCorrection' : 'adjustment',
		leaveTypeID: centralLeaveType.id,
		leaveTypeName: centralLeaveType.name,
		deltaMilliDays: Math.round((entry.delta_days ?? 0) * 1000),
		effectiveOn: entry.effective_on,
		occurredAt: entry.occurred_at,
		reason: entry.reason ?? undefined
	};
}

export function leaveRowsByMemberID(
	rows: readonly SupabaseLeaveManagementRow[]
): Map<string, SupabaseLeaveManagementRow[]> {
	const grouped = new Map<string, SupabaseLeaveManagementRow[]>();
	for (const row of rows) grouped.set(row.member_id, [...(grouped.get(row.member_id) ?? []), row]);
	return grouped;
}

function yearOf(row: SupabaseLeaveManagementRow, timeZone: string): number {
	return Number(leaveDisplayRange(row.starts_at, row.ends_at, row.days, timeZone).startDate.slice(0, 4));
}

function milliDays(rows: readonly SupabaseLeaveManagementRow[]): number {
	return rows.reduce((total, row) => total + Math.round(row.days * 1000), 0);
}

function unitOf(days: number): EmployeeLeaveUnit {
	if (days <= 0.25) return 'quarterDay';
	if (days <= 0.5) return 'halfDay';
	return 'fullDay';
}
