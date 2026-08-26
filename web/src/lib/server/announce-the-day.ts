import type { SupabaseClient } from '@supabase/supabase-js';
import { dayIn, whoseHourItIs, type Listener } from '$lib/notifications/day-digest-timing';
import { notifyMember, type Notification } from './notify-member';
import type { VapidKeys } from './web-push-vapid';

type MemberRow = { id: string; timezone: string | null; notification_settings: unknown };
type CompanyRow = { id: string; timezone: string };
type EventRow = {
	id: string;
	title: string;
	starts_at: string;
	ends_at: string | null;
	is_whole_day: boolean;
	task_participant: { member_id: string }[];
};

export type Announced = { told: number; reached: number };

export async function announceTheDay(
	record: SupabaseClient,
	moment: Date,
	vapid: VapidKeys,
	nowInSeconds: number
): Promise<Announced> {
	let told = 0;
	let reached = 0;
	for (const company of await companies(record)) {
		const listening = whoseHourItIs(await listenersOf(record, company), moment);
		if (listening.length === 0) continue;

		const events = await eventsOn(record, company.id, dayIn(listening[0].timeZone, moment), listening[0].timeZone);
		for (const listener of listening) {
			const mine = events.filter((event) => isFor(event, listener.memberID));
			if (mine.length === 0) continue;
			const delivery = await notifyMember(
				record,
				listener.memberID,
				'calendar',
				dayNotification(mine, listener.timeZone),
				vapid,
				nowInSeconds
			);
			if (!delivery.silent) told += 1;
			reached += delivery.reached;
		}
	}
	return { told, reached };
}

async function companies(record: SupabaseClient): Promise<CompanyRow[]> {
	const { data, error } = await record.from('company').select('id, timezone').returns<CompanyRow[]>();
	if (error) throw new Error(error.message);
	return data ?? [];
}

async function listenersOf(record: SupabaseClient, company: CompanyRow): Promise<Listener[]> {
	const { data, error } = await record
		.from('member')
		.select('id, timezone, notification_settings')
		.eq('company_id', company.id)
		.eq('status', 'active')
		.returns<MemberRow[]>();
	if (error) throw new Error(error.message);
	return (data ?? []).map((member) => ({
		memberID: member.id,
		timeZone: member.timezone ?? company.timezone,
		notificationSettings: member.notification_settings
	}));
}

async function eventsOn(
	record: SupabaseClient,
	companyID: string,
	day: string,
	timeZone: string
): Promise<EventRow[]> {
	const { data, error } = await record
		.from('task')
		.select('id, title, starts_at, ends_at, is_whole_day, task_participant (member_id)')
		.eq('company_id', companyID)
		.eq('is_event', true)
		.lt('starts_at', endOfDay(day, timeZone))
		.gte('ends_at', startOfDay(day, timeZone))
		.order('starts_at')
		.returns<EventRow[]>();
	if (error) throw new Error(error.message);
	return data ?? [];
}

function startOfDay(day: string, timeZone: string): string {
	return new Date(`${day}T00:00:00${offsetOf(day, timeZone)}`).toISOString();
}

function endOfDay(day: string, timeZone: string): string {
	return new Date(`${day}T23:59:59${offsetOf(day, timeZone)}`).toISOString();
}

function offsetOf(day: string, timeZone: string): string {
	const named = new Intl.DateTimeFormat('en-US', { timeZone, timeZoneName: 'longOffset' })
		.formatToParts(new Date(`${day}T12:00:00Z`))
		.find((part) => part.type === 'timeZoneName');
	const said = /GMT([+-]\d{2}:\d{2})/.exec(named?.value ?? '');
	return said ? said[1] : 'Z';
}

function isFor(event: EventRow, memberID: string): boolean {
	return (event.task_participant ?? []).some((participant) => participant.member_id === memberID);
}

function dayNotification(events: EventRow[], timeZone: string): Notification {
	return {
		title: `오늘 일정 ${events.length}건`,
		body: events.map((event) => `${startTime(event, timeZone)} ${event.title}`.trim()).join('\n'),
		openPath: '/calendar/',
		tag: 'today'
	};
}

function startTime(event: EventRow, timeZone: string): string {
	if (event.is_whole_day) return '종일';
	const moment = new Date(event.starts_at);
	if (Number.isNaN(moment.getTime())) return '';
	return new Intl.DateTimeFormat('en-GB', { timeZone, hour: '2-digit', minute: '2-digit', hour12: false }).format(moment);
}
