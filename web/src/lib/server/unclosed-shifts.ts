import type { SupabaseClient } from '@supabase/supabase-js';
import { companyDateOf, companyTimeOf } from '$lib/company-time';
import type { Environment } from './agent-request';
import { tell } from './tell';

export type AttendanceEventRow = {
	id: string;
	member_id: string;
	kind: 'clock_in' | 'clock_out';
	occurred_at: string;
	asked_at: string | null;
};

export type UnclosedShift = {
	eventID: string;
	memberID: string;
	occurredAt: string;
};

export type Asking = {
	asked: number;
	failures: string[];
};

const hoursAShiftMayLast = 24;
const hoursAShiftStaysWorthAskingAbout = 48;

export function shiftsNobodyClosed(rows: AttendanceEventRow[], now: Date): UnclosedShift[] {
	const latestByMember = new Map<string, AttendanceEventRow>();
	for (const row of rows) {
		const held = latestByMember.get(row.member_id);
		if (!held || held.occurred_at < row.occurred_at) latestByMember.set(row.member_id, row);
	}
	return [...latestByMember.values()]
		.filter((row) => row.kind === 'clock_in' && row.asked_at === null)
		.filter((row) => hoursSince(row.occurred_at, now) > hoursAShiftMayLast)
		.map((row) => ({ eventID: row.id, memberID: row.member_id, occurredAt: row.occurred_at }))
		.sort((left, right) => left.occurredAt.localeCompare(right.occurredAt));
}

export async function askAboutUnclosedShifts(
	environment: Environment,
	record: SupabaseClient,
	now: Date
): Promise<Asking> {
	const rows = await attendanceSince(record, hoursBefore(now, hoursAShiftStaysWorthAskingAbout));
	const shifts = shiftsNobodyClosed(rows, now);
	const timeZones = await timeZoneByMember(record, shifts.map((shift) => shift.memberID));
	const failures: string[] = [];
	let asked = 0;
	for (const shift of shifts) {
		const failure = await askAbout(environment, record, shift, timeZones.get(shift.memberID), now);
		if (failure) failures.push(`${shift.eventID}: ${failure}`);
		else asked += 1;
	}
	return { asked, failures };
}

async function askAbout(
	environment: Environment,
	record: SupabaseClient,
	shift: UnclosedShift,
	timeZone: string | undefined,
	now: Date
): Promise<string> {
	if (!timeZone) return `member ${shift.memberID} belongs to no company with a time zone`;
	const told = await tell(environment, tellingAbout(shift, timeZone), record);
	if (!told.pushed && !told.messaged) return told.failure ?? 'nobody was reached';
	await markAsked(record, shift.eventID, now);
	return '';
}

export function tellingAbout(shift: UnclosedShift, timeZone: string) {
	return {
		memberID: shift.memberID,
		category: 'attendance' as const,
		title: '퇴근 기록이 없습니다',
		body: `${startedAtInWords(shift.occurredAt, timeZone)} 출근 후 퇴근 기록이 없습니다. 퇴근 시각을 알려주세요.`
	};
}

function startedAtInWords(occurredAt: string, timeZone: string): string {
	const started = new Date(occurredAt);
	if (Number.isNaN(started.getTime())) return occurredAt;
	return `${companyDateOf(started, timeZone)} ${companyTimeOf(started, timeZone)}`;
}

async function timeZoneByMember(
	record: SupabaseClient,
	memberIDs: string[]
): Promise<Map<string, string>> {
	if (memberIDs.length === 0) return new Map();
	const { data, error } = await record
		.from('member')
		.select('id, company (timezone)')
		.in('id', memberIDs)
		.returns<{ id: string; company: { timezone: string } | null }[]>();
	if (error) throw new Error(error.message);
	return new Map(data.flatMap((row) => (row.company ? [[row.id, row.company.timezone] as const] : [])));
}

async function attendanceSince(record: SupabaseClient, since: Date): Promise<AttendanceEventRow[]> {
	const { data, error } = await record
		.from('attendance')
		.select('id, member_id, kind, occurred_at, asked_at')
		.is('deleted_at', null)
		.gte('occurred_at', since.toISOString())
		.order('occurred_at')
		.returns<AttendanceEventRow[]>();
	if (error) throw new Error(error.message);
	return data;
}

async function markAsked(record: SupabaseClient, eventID: string, now: Date): Promise<void> {
	const { error } = await record
		.from('attendance')
		.update({ asked_at: now.toISOString() })
		.eq('id', eventID);
	if (error) throw new Error(error.message);
}

function hoursSince(occurredAt: string, now: Date): number {
	return (now.getTime() - new Date(occurredAt).getTime()) / (60 * 60 * 1000);
}

function hoursBefore(now: Date, hours: number): Date {
	return new Date(now.getTime() - hours * 60 * 60 * 1000);
}
