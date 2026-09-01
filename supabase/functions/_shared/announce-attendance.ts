import type { SupabaseClient } from './service-client.ts';
import { notifyMember, type Notification } from './notify-member.ts';
import { whoAnswersFor } from './who-answers.ts';
import type { VapidKeys } from './web-push-vapid.ts';

export type Member = {
	id: string;
	name: string | null;
	company_id: string;
	timezone: string | null;
	company: { timezone: string } | null;
};
export type ClockRow = { id: string; kind: string; location: string | null; occurred_at: string };
export type LeaveRow = { id: string; kind: string; starts_at: string; ends_at: string | null };

export type Announced = { told: number; reached: number };

export async function announceClock(
	caller: SupabaseClient,
	record: SupabaseClient,
	memberID: string,
	vapid: VapidKeys,
	nowInSeconds: number
): Promise<Announced> {
	const clocked = await newestClock(caller, memberID);
	if (!clocked) return { told: 0, reached: 0 };

	const announcer = await memberOf(record, memberID);
	const notification: Notification = {
		title: `${nameOf(announcer)} ${clocked.kind === 'clock_in' ? '출근' : '퇴근'}`,
		body: clockBody(clocked, zoneOf(announcer)),
		openPath: '/attendance/',
		tag: `attendance-${clocked.id}`
	};
	return tellEachExcept(record, announcer.company_id, memberID, 'attendance', notification, vapid, nowInSeconds);
}

export async function announceLeaveRequest(
	caller: SupabaseClient,
	record: SupabaseClient,
	memberID: string,
	vapid: VapidKeys,
	nowInSeconds: number
): Promise<Announced> {
	const asked = await newestLeaveRequest(caller, memberID);
	if (!asked) return { told: 0, reached: 0 };

	const announcer = await memberOf(record, memberID);
	const notification: Notification = {
		title: `휴가 신청: ${nameOf(announcer)}`,
		body: leaveBody(asked, zoneOf(announcer)),
		openPath: '/attendance/',
		tag: `leave-${asked.id}`
	};
	return tellWhoAnswers(record, announcer.company_id, memberID, notification, vapid, nowInSeconds);
}

async function newestClock(caller: SupabaseClient, memberID: string): Promise<ClockRow | null> {
	const { data, error } = await caller
		.from('attendance')
		.select('id, kind, location, occurred_at')
		.eq('member_id', memberID)
		.order('occurred_at', { ascending: false })
		.limit(1)
		.maybeSingle<ClockRow>();
	if (error) throw new Error(error.message);
	return data;
}

async function newestLeaveRequest(caller: SupabaseClient, memberID: string): Promise<LeaveRow | null> {
	const { data, error } = await caller
		.from('leave')
		.select('id, kind, starts_at, ends_at')
		.eq('member_id', memberID)
		.eq('status', 'requested')
		.order('starts_at', { ascending: false })
		.limit(1)
		.maybeSingle<LeaveRow>();
	if (error) throw new Error(error.message);
	return data;
}

async function memberOf(record: SupabaseClient, memberID: string): Promise<Member> {
	const { data, error } = await record
		.from('member')
		.select('id, name, company_id, timezone, company (timezone)')
		.eq('id', memberID)
		.single<Member>();
	if (error) throw new Error(error.message);
	return data;
}

export function zoneOf(member: Member): string {
	const timeZone = member.timezone ?? member.company?.timezone ?? '';
	if (!timeZone) throw new Error(`member ${member.id} keeps no time zone and belongs to no company with one`);
	return timeZone;
}

async function tellEachExcept(
	record: SupabaseClient,
	companyID: string,
	announcerID: string,
	category: 'attendance',
	notification: Notification,
	vapid: VapidKeys,
	nowInSeconds: number
): Promise<Announced> {
	const listeners = await companyMembers(record, companyID);
	return tellEach(record, listeners.filter((id) => id !== announcerID), category, notification, vapid, nowInSeconds);
}

async function tellWhoAnswers(
	record: SupabaseClient,
	companyID: string,
	announcerID: string,
	notification: Notification,
	vapid: VapidKeys,
	nowInSeconds: number
): Promise<Announced> {
	const answering = await whoAnswersFor(record, companyID, announcerID);
	return tellEach(record, answering, 'leave', notification, vapid, nowInSeconds);
}

async function companyMembers(record: SupabaseClient, companyID: string): Promise<string[]> {
	const { data, error } = await record
		.from('member')
		.select('id')
		.eq('company_id', companyID)
		.neq('status', 'withdrawn')
		.returns<{ id: string }[]>();
	if (error) throw new Error(error.message);
	return (data ?? []).map((member) => member.id);
}

async function tellEach(
	record: SupabaseClient,
	memberIDs: string[],
	category: 'attendance' | 'leave',
	notification: Notification,
	vapid: VapidKeys,
	nowInSeconds: number
): Promise<Announced> {
	let told = 0;
	let reached = 0;
	for (const memberID of memberIDs) {
		const delivery = await notifyMember(record, memberID, category, notification, vapid, nowInSeconds);
		if (!delivery.silent) told += 1;
		reached += delivery.reached;
	}
	return { told, reached };
}

function nameOf(member: Member): string {
	return (member.name ?? '').trim() || '누군가';
}

export function clockBody(clocked: ClockRow, timeZone: string): string {
	const where = (clocked.location ?? '').trim();
	const at = clockTime(clocked.occurred_at, timeZone);
	if (where && at) return `${where} · ${at}`;
	return where || at;
}

function clockTime(occurredAt: string, timeZone: string): string {
	const moment = new Date(occurredAt);
	if (Number.isNaN(moment.getTime())) return '';
	return new Intl.DateTimeFormat('en-GB', {
		timeZone,
		hour: '2-digit',
		minute: '2-digit',
		hourCycle: 'h23'
	}).format(moment);
}

export function leaveBody(asked: LeaveRow, timeZone: string): string {
	const starts = dayOf(asked.starts_at, timeZone);
	const ends = lastDayOff(asked.ends_at, timeZone);
	const when = ends > starts ? `${starts} ~ ${ends}` : starts;
	return [asked.kind, when].filter((part) => part !== '').join(' ');
}

function dayOf(value: string, timeZone: string): string {
	const moment = new Date(value);
	if (Number.isNaN(moment.getTime())) return '';
	return new Intl.DateTimeFormat('en-CA', {
		timeZone,
		year: 'numeric',
		month: '2-digit',
		day: '2-digit'
	}).format(moment);
}

function lastDayOff(endsAt: string | null, timeZone: string): string {
	if (endsAt === null) return '';
	const ended = new Date(endsAt);
	if (Number.isNaN(ended.getTime())) return '';
	return dayOf(new Date(ended.getTime() - 1).toISOString(), timeZone);
}
