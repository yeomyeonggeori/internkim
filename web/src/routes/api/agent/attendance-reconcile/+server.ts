import { error, json } from '@sveltejs/kit';
import type { SupabaseClient } from '@supabase/supabase-js';
import { callingAgent, environmentOf } from '$lib/server/agent-request';
import { membersOfCompanyByExternalID } from '$lib/server/member-credential';
import {
	attendanceWorkCalendarFromDevice,
	attendanceWorkModeFromDevice,
	InvalidAttendanceWorkCalendarError,
	InvalidAttendanceWorkModeError,
	saveAttendanceWorkCalendar
} from '$lib/server/attendance-work-calendar-reconcile';
import {
	EmptyWindowRefused,
	reconcileMember,
	type Reconciliation,
	type DeviceAttendance,
	type RecordedAttendance
} from '$lib/server/attendance-reconcile';
import type { RequestHandler } from './$types';

type ReconcileRequest = {
	platform?: unknown;
	workMode?: unknown;
	workCalendar?: unknown;
	from?: unknown;
	to?: unknown;
	events?: unknown;
};

type OfferedEvent = {
	externalID?: unknown;
	kind?: unknown;
	occurredAt?: unknown;
	location?: unknown;
};

export const POST: RequestHandler = async ({ request, platform }) => {
	const { client, companyID } = await callingAgent(request, environmentOf(platform));

	const asked = (await request.json().catch(() => ({}))) as ReconcileRequest;
	const from = moment(asked.from, 'from');
	const to = moment(asked.to, 'to');
	if (from >= to) error(400, 'the window ends before it begins');
	askedWorkMode(asked.workMode);
	const workCalendar = askedWorkCalendar(asked.workCalendar);
	if (workCalendar !== undefined) {
		await saveAttendanceWorkCalendar(client, companyID, workCalendar);
	}

	const memberOf = await membersOfCompanyByExternalID(client, companyID, askedPlatform(asked.platform));
	const byMember = groupByMember(asked.events, memberOf, from, to);
	const held = await heldInWindow(client, [...memberOf.values()], from, to);

	return json(await makeTheRecordMatch(client, new Set(memberOf.values()), byMember, held));
};

function askedWorkCalendar(offered: unknown) {
	try {
		return attendanceWorkCalendarFromDevice(offered);
	} catch (thrown) {
		if (!(thrown instanceof InvalidAttendanceWorkCalendarError)) throw thrown;
		error(400, thrown.message);
	}
}

function askedWorkMode(offered: unknown) {
	try {
		return attendanceWorkModeFromDevice(offered);
	} catch (thrown) {
		if (!(thrown instanceof InvalidAttendanceWorkModeError)) throw thrown;
		error(400, thrown.message);
	}
}

async function makeTheRecordMatch(
	client: SupabaseClient,
	memberIDs: Set<string>,
	byMember: Map<string, DeviceAttendance[]>,
	held: Map<string, RecordedAttendance[]>
): Promise<{ added: number; removed: number; refused: string[]; rejected: string[] }> {
	let added = 0;
	let removed = 0;
	const refused: string[] = [];
	const rejected: string[] = [];

	for (const memberID of memberIDs) {
		try {
			const plan = reconcileMember(memberID, byMember.get(memberID) ?? [], held.get(memberID) ?? []);
			if (plan.remove.length > 0) await remove(client, plan.remove);
			const kept = await insertEachThatTheRecordTakes(client, plan.add);
			added += kept.added;
			rejected.push(...kept.rejected);
			removed += plan.remove.length;
		} catch (thrown) {
			if (!(thrown instanceof EmptyWindowRefused)) throw thrown;
			refused.push(memberID);
		}
	}
	return { added, removed, refused, rejected };
}

async function insertEachThatTheRecordTakes(
	client: SupabaseClient,
	rows: Reconciliation['add']
): Promise<{ added: number; rejected: string[] }> {
	if (rows.length === 0) return { added: 0, rejected: [] };
	try {
		await insert(client, rows);
		return { added: rows.length, rejected: [] };
	} catch {
		return insertOneByOne(client, rows);
	}
}

async function insertOneByOne(
	client: SupabaseClient,
	rows: Reconciliation['add']
): Promise<{ added: number; rejected: string[] }> {
	let added = 0;
	const rejected: string[] = [];
	for (const row of rows) {
		try {
			await insert(client, [row]);
			added += 1;
		} catch (thrown) {
			const said = thrown instanceof Error ? thrown.message : 'refused';
			rejected.push(`${row.occurred_at} ${row.kind} ${row.location ?? '-'}: ${said}`);
		}
	}
	return { added, rejected };
}

function askedPlatform(offered: unknown): string {
	if (typeof offered !== 'string' || !offered.trim()) error(400, 'which messenger these people are on');
	return offered.trim();
}

function moment(offered: unknown, named: string): string {
	if (typeof offered !== 'string' || Number.isNaN(Date.parse(offered))) error(400, `${named} must be a time`);
	return new Date(offered).toISOString();
}

function groupByMember(
	offered: unknown,
	memberOf: Map<string, string>,
	from: string,
	to: string
): Map<string, DeviceAttendance[]> {
	const byMember = new Map<string, DeviceAttendance[]>();
	if (!Array.isArray(offered)) return byMember;

	for (const entry of offered as OfferedEvent[]) {
		const memberID = typeof entry.externalID === 'string' ? memberOf.get(entry.externalID) : undefined;
		if (!memberID) continue;
		if (typeof entry.kind !== 'string' || typeof entry.occurredAt !== 'string') continue;
		if (Number.isNaN(Date.parse(entry.occurredAt))) continue;
		const moment = new Date(entry.occurredAt).toISOString();
		if (moment < from || moment >= to) continue;
		const forMember = byMember.get(memberID) ?? [];
		forMember.push({
			kind: entry.kind,
			occurredAt: moment,
			location: typeof entry.location === 'string' ? entry.location : ''
		});
		byMember.set(memberID, forMember);
	}
	return byMember;
}

async function heldInWindow(
	client: SupabaseClient,
	memberIDs: string[],
	from: string,
	to: string
): Promise<Map<string, RecordedAttendance[]>> {
	const byMember = new Map<string, RecordedAttendance[]>();
	if (memberIDs.length === 0) return byMember;

	const { data, error: failed } = await client
		.from('attendance')
		.select('id, member_id, kind, occurred_at')
		.in('member_id', memberIDs)
		.gte('occurred_at', from)
		.lt('occurred_at', to)
		.returns<(RecordedAttendance & { member_id: string })[]>();
	if (failed) throw new Error(failed.message);

	for (const row of data ?? []) {
		const forMember = byMember.get(row.member_id) ?? [];
		forMember.push(row);
		byMember.set(row.member_id, forMember);
	}
	return byMember;
}

async function insert(client: SupabaseClient, rows: { member_id: string }[]): Promise<void> {
	const { error: failed } = await client.from('attendance').insert(rows);
	if (failed) throw new Error(failed.message);
}

async function remove(client: SupabaseClient, ids: string[]): Promise<void> {
	const { error: failed } = await client.from('attendance').delete().in('id', ids);
	if (failed) throw new Error(failed.message);
}
