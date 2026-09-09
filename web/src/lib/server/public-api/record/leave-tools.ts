import { leaveYearOf } from '$lib/attendance/leave-year-share';
import { dayIn, dayOfInstant, dayShifted, instantOfDay, instantWritten } from './days';
import { personName } from '$lib/person-name';
import { personOfHint } from './people';
import { ownerNamedBy, whoseRecords, whoseRecordsHolds } from './whose';
import { statusOfPostgresCode, RecordRefusedTheWrite } from './tasks';
import {
	leaveBalancesOfCompany,
	leaveDaysGranted,
	leaveDaysRemaining,
	leaveKindOf,
	leaveOfCompany,
	NoSuchLeave,
	type LeaveRow
} from './leave';
import type { RecordContext } from './company';

export type AnsweredLeave = {
	leaveID: string;
	personID: string;
	person: string;
	kindID: string;
	kind: string;
	days: number;
	status: string;
	isPaid: boolean;
	isDeducted: boolean;
	startDate: string;
	endDate: string;
	startsAt: string;
	endsAt: string;
	note: string | null;
};

const halfADay = 0.5;

function midnightAfter(timezone: string, endsAt: string): string {
	const lastDay = dayIn(timezone, new Date(instantWritten(timezone, endsAt, true)));
	return instantOfDay(timezone, dayShifted(lastDay, 1));
}

function lastDayCovered(timezone: string, row: LeaveRow): string {
	const ends = dayOfInstant(timezone, row.ends_at);
	return Number(row.days) > halfADay ? dayShifted(ends, -1) : ends;
}

function answeredLeave(context: RecordContext, row: LeaveRow): AnsweredLeave {
	const nameOf = new Map(context.people.map((person) => [person.personID, personName(person.name, context.locale)]));
	return {
		leaveID: row.id,
		personID: row.member_id,
		person: nameOf.get(row.member_id) ?? row.member_id,
		kindID: row.kind,
		kind: context.leaveKinds.find((kind) => kind.id === row.kind)?.name ?? row.kind,
		days: Number(row.days),
		status: row.status,
		isPaid: row.is_paid,
		isDeducted: row.is_deducted,
		startDate: dayOfInstant(context.labels.timezone, row.starts_at),
		endDate: lastDayCovered(context.labels.timezone, row),
		startsAt: new Date(row.starts_at).toISOString(),
		endsAt: new Date(row.ends_at).toISOString(),
		note: row.note
	};
}

function describedLeave(context: RecordContext, row: LeaveRow): string {
	const answered = answeredLeave(context, row);
	return `${answered.person} · ${answered.kind} · ${answered.startDate}`;
}

function leaveOfHint(context: RecordContext, rows: LeaveRow[], hint: string): LeaveRow {
	const asked = hint.trim();
	if (!asked) throw new NoSuchLeave(hint, []);

	const byID = rows.find((row) => row.id === asked);
	if (byID) return byID;

	const described = rows.filter((row) => describedLeave(context, row).includes(asked));
	if (described.length === 1) return described[0];
	throw new NoSuchLeave(hint, described.map((row) => describedLeave(context, row)));
}

async function leaveByID(context: RecordContext, leaveID: string): Promise<LeaveRow> {
	const written = (await leaveOfCompany(context.caller)).find((row) => row.id === leaveID);
	if (!written) throw new NoSuchLeave(leaveID, []);
	return written;
}

function personNameOf(context: RecordContext, personID: string): string {
	if (!personID) return '';
	return personName(context.people.find((person) => person.personID === personID)?.name ?? '', context.locale);
}

function targetMember(context: RecordContext, personHint: string | undefined): string {
	return personHint ? personOfHint(context.people, personHint).personID : context.requesterID;
}

export type LeaveListInput = {
	personHints?: string[];
	scope?: string;
	from?: string;
	to?: string;
	status?: string;
	limit?: number;
};

export async function leaveList(context: RecordContext, input: LeaveListInput) {
	const whose = whoseRecords(context.people, input.personHints, input.scope, context.requesterID);
	const from = input.from ? dayIn(context.labels.timezone, new Date(instantWritten(context.labels.timezone, input.from))) : '';
	const to = input.to ? dayIn(context.labels.timezone, new Date(instantWritten(context.labels.timezone, input.to, true))) : '';

	const rows = (await leaveOfCompany(context.caller)).filter((row) => {
		if (!whoseRecordsHolds(whose, row.member_id)) return false;
		if (input.status && row.status !== input.status) return false;
		const startDate = dayOfInstant(context.labels.timezone, row.starts_at);
		const endDate = dayOfInstant(context.labels.timezone, row.ends_at);
		if (from && endDate < from) return false;
		if (to && startDate > to) return false;
		return true;
	});

	const kept = input.limit && input.limit > 0 ? rows.slice(0, input.limit) : rows;
	return {
		scope: whose.everyone ? 'everyone' : 'person',
		personID: ownerNamedBy(whose) || null,
		personName: personNameOf(context, ownerNamedBy(whose)),
		statusFilter: input.status ?? null,
		count: kept.length,
		leave: kept.map((row) => answeredLeave(context, row)),
		registeredKinds: context.leaveKinds.map((kind) => kind.name)
	};
}

export type AnsweredBalance = {
	personID: string;
	personName: string;
	grantedDays: number | null;
	remainingDays: number | null;
	usedDays: number | null;
	tracking: string;
};

function answeredBalance(
	context: RecordContext,
	memberID: string,
	granted: number | null,
	remaining: number | null
): AnsweredBalance {
	return {
		personID: memberID,
		personName: personNameOf(context, memberID),
		grantedDays: granted,
		remainingDays: remaining,
		usedDays: granted === null || remaining === null ? null : Number((granted - remaining).toFixed(2)),
		tracking: granted === null ? 'unlimited' : 'managed'
	};
}

async function balanceOfOne(
	context: RecordContext,
	memberID: string,
	year: number
): Promise<AnsweredBalance> {
	const granted = await leaveDaysGranted(context.caller, memberID);
	const remaining = await leaveDaysRemaining(context.caller, memberID, year);
	return answeredBalance(context, memberID, granted, remaining);
}

export type LeaveBalanceInput = { personHints?: string[]; scope?: string; year?: number };

export async function leaveBalance(context: RecordContext, input: LeaveBalanceInput) {
	const accrued = await context.caller.rpc('leave_accrue_due');
	if (accrued.error) throw new Error(accrued.error.message);
	const year =
		input.year ??
		leaveYearOf(
			dayIn(context.labels.timezone, context.now),
			context.leaveYearStart.month,
			context.leaveYearStart.day
		);
	const whose = whoseRecords(context.people, input.personHints, input.scope, context.requesterID);
	const balances = whose.everyone
		? (await leaveBalancesOfCompany(context.caller, year)).map((row) =>
				answeredBalance(context, row.memberID, row.grantedDays, row.remainingDays)
			)
		: await Promise.all(whose.personIDs.map((personID) => balanceOfOne(context, personID, year)));

	return {
		scope: whose.everyone ? 'everyone' : 'person',
		year,
		count: balances.length,
		balances
	};
}

export type LeaveGrantSetInput = { personHint?: string; days?: number };

export async function leaveGrantSet(
	context: RecordContext,
	input: LeaveGrantSetInput
): Promise<AnsweredBalance> {
	if (!input.personHint?.trim()) throw new Error('a leave grant names the person it is for');
	if (input.days === undefined || input.days < 0) {
		throw new Error('a leave grant is a number of days, and never fewer than none');
	}

	const memberID = personOfHint(context.people, input.personHint).personID;
	const { error } = await context.caller.rpc('member_leave_days_set', {
		target_member: memberID,
		granted_days: input.days
	});
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));

	const year = Number(dayIn(context.labels.timezone, context.now).slice(0, 4));
	return balanceOfOne(context, memberID, year);
}

export type LeaveReturnEarlyInput = { location?: string };

type ReturnedEarly = {
	shortened?: boolean;
	leaveID?: string | null;
	endsAt?: string | null;
	days?: number | string | null;
};

export async function leaveReturnEarly(context: RecordContext, input: LeaveReturnEarlyInput) {
	const { data, error } = await context.caller.rpc('leave_return_early', {
		work_location: input.location?.trim() || ''
	});
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));

	const answered = (data ?? {}) as ReturnedEarly;
	return {
		shortened: answered.shortened === true,
		leaveID: answered.leaveID ?? null,
		endsAt: answered.endsAt ? new Date(answered.endsAt).toISOString() : null,
		days: answered.days === null || answered.days === undefined ? null : Number(answered.days)
	};
}

export type LeaveRequestInput = {
	personHint?: string;
	kind?: string;
	startsAt?: string;
	endsAt?: string;
	days?: number;
	note?: string;
};

export async function leaveRequest(
	context: RecordContext,
	input: LeaveRequestInput
): Promise<AnsweredLeave> {
	if (!input.kind?.trim()) throw new Error('a leave request names the kind of leave it is');
	if (!input.startsAt || !input.endsAt) throw new Error('a leave request names the days it covers');
	if (input.days === undefined || input.days <= 0) {
		throw new Error('a leave request says how many days it consumes; a half day is 0.5');
	}

	const kind = leaveKindOf(context.leaveKinds, input.kind);
	const written = {
		member_id: targetMember(context, input.personHint),
		kind: kind.id,
		is_paid: kind.isPaid,
		is_deducted: kind.isDeducted,
		days: input.days,
		status: 'requested',
		starts_at: instantWritten(context.labels.timezone, input.startsAt),
		ends_at:
			input.days > halfADay
				? midnightAfter(context.labels.timezone, input.endsAt)
				: instantWritten(context.labels.timezone, input.endsAt, true),
		note: input.note?.trim() || null
	};

	const { data, error } = await context.caller.from('leave').insert(written).select('id').single<{ id: string }>();
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));
	return answeredLeave(context, await leaveByID(context, data.id));
}

export type LeaveUpdateInput = {
	leaveHint?: string;
	kind?: string;
	startsAt?: string;
	endsAt?: string;
	days?: number;
	note?: string;
};

export async function leaveUpdate(
	context: RecordContext,
	input: LeaveUpdateInput
): Promise<AnsweredLeave> {
	if (!input.leaveHint) throw new Error('a correction names the leave it corrects');
	if (input.days !== undefined && input.days <= 0) {
		throw new Error('a leave consumes more than nothing; a half day is 0.5');
	}

	const row = leaveOfHint(context, await leaveOfCompany(context.caller), input.leaveHint);
	const held = answeredLeave(context, row);
	const days = input.days ?? held.days;
	const corrected: Record<string, unknown> = {
		days,
		starts_at: instantWritten(context.labels.timezone, input.startsAt ?? held.startDate),
		ends_at:
			days > halfADay
				? midnightAfter(context.labels.timezone, input.endsAt ?? held.endDate)
				: instantWritten(context.labels.timezone, input.endsAt ?? held.endDate, true)
	};
	if (input.kind?.trim()) {
		const kind = leaveKindOf(context.leaveKinds, input.kind);
		corrected.kind = kind.id;
		corrected.is_paid = kind.isPaid;
		corrected.is_deducted = kind.isDeducted;
	}
	if (input.note !== undefined) corrected.note = input.note.trim() || null;

	const { error } = await context.caller
		.from('leave')
		.update(corrected)
		.eq('id', row.id)
		.select('id')
		.single<{ id: string }>();
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));
	return answeredLeave(context, await leaveByID(context, row.id));
}

export type LeaveDeleteInput = { leaveHint?: string };

export async function leaveDelete(
	context: RecordContext,
	input: LeaveDeleteInput
): Promise<AnsweredLeave> {
	if (!input.leaveHint) throw new Error('a removal names the leave it removes');

	const row = leaveOfHint(context, await leaveOfCompany(context.caller), input.leaveHint);
	const taken = answeredLeave(context, row);
	const { error } = await context.caller
		.from('leave')
		.delete()
		.eq('id', row.id)
		.select('id')
		.single<{ id: string }>();
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));
	return taken;
}

export type LeaveDecideInput = { leaveHint?: string; decision?: string };

export async function leaveDecide(
	context: RecordContext,
	input: LeaveDecideInput
): Promise<AnsweredLeave> {
	if (!input.leaveHint) throw new Error('a decision names the leave it decides');
	if (input.decision !== 'approved' && input.decision !== 'rejected') {
		throw new Error('a decision is approved or rejected');
	}

	const rows = await leaveOfCompany(context.caller);
	const row = leaveOfHint(context, rows, input.leaveHint);
	const { error } = await context.caller
		.from('leave')
		.update({ status: input.decision })
		.eq('id', row.id)
		.select('id')
		.single<{ id: string }>();
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));
	return answeredLeave(context, await leaveByID(context, row.id));
}
