import type { SupabaseClient } from '@supabase/supabase-js';
import { defaultLeavePolicy } from '$lib/attendance/leave-policy-defaults';
import type { AttendanceLeavePolicy } from '../../../../routes/admin/admin-types';

export type LeaveRow = {
	id: string;
	member_id: string;
	kind: string;
	is_paid: boolean;
	is_deducted: boolean;
	days: number;
	status: string;
	starts_at: string;
	ends_at: string;
	note: string | null;
};

export type LeaveKind = {
	id: string;
	name: string;
	isPaid: boolean;
	isDeducted: boolean;
};

export class NoSuchLeaveKind extends Error {
	constructor(
		readonly asked: string,
		readonly registered: string[]
	) {
		super(
			registered.length === 0
				? 'this company registers no leave types'
				: `${asked} is not one of ${registered.join(', ')}`
		);
		this.name = 'NoSuchLeaveKind';
	}
}

export class NoSuchLeave extends Error {
	constructor(
		readonly hint: string,
		readonly candidates: string[]
	) {
		super(
			candidates.length === 0
				? `no leave here goes by ${hint}`
				: `${hint} could be ${candidates.join(' / ')}; name one of them exactly`
		);
		this.name = 'NoSuchLeave';
	}
}

export function leaveKindsOfPolicy(rules: unknown): LeaveKind[] {
	const held = (rules ?? {}) as { attendanceLeavePolicy?: AttendanceLeavePolicy };
	const policy = held.attendanceLeavePolicy ?? defaultLeavePolicy();
	return policy.leaveTypes
		.filter((leaveType) => leaveType.isActive)
		.map((leaveType) => ({
			id: leaveType.id,
			name: leaveType.name,
			isPaid: leaveType.paid,
			isDeducted: leaveType.balanceMode === 'annual'
		}));
}

export function leaveKindOf(kinds: LeaveKind[], asked: string): LeaveKind {
	const written = asked.trim();
	if (!written) throw new NoSuchLeaveKind(asked, kinds.map((kind) => kind.name));

	const exact = kinds.find(
		(kind) => kind.id === written || kind.name.toLowerCase() === written.toLowerCase()
	);
	if (exact) return exact;

	const contained = kinds.filter((kind) => kind.name.includes(written));
	if (contained.length === 1) return contained[0];
	throw new NoSuchLeaveKind(asked, kinds.map((kind) => kind.name));
}

export async function leaveOfCompany(caller: SupabaseClient): Promise<LeaveRow[]> {
	const { data, error } = await caller.rpc('leave_in_full');
	if (error) throw new Error(error.message);
	const rows = (data ?? []) as LeaveRow[];
	return [...rows].sort((left, right) => right.starts_at.localeCompare(left.starts_at));
}

export async function leaveDaysGranted(caller: SupabaseClient, memberID: string): Promise<number | null> {
	const { data, error } = await caller.rpc('member_leave_days', { target_member: memberID });
	if (error) throw new Error(error.message);
	return data === null ? null : Number(data);
}

export async function leaveDaysRemaining(
	caller: SupabaseClient,
	memberID: string,
	year: number
): Promise<number | null> {
	const { data, error } = await caller.rpc('member_leave_remaining', {
		target_member: memberID,
		target_year: year
	});
	if (error) throw new Error(error.message);
	return data === null ? null : Number(data);
}
