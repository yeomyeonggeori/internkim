import type { SupabaseClient } from '@supabase/supabase-js';
import { dayOffColor } from './day-off-color';
import { localizedLeaveUnitName } from '../i18n/leave-type-name';
import type { Locale } from '../i18n/locale.svelte';
import type { CalendarEvent } from '../../routes/calendar/embed/calendar-event-persistence';

export type CalendarLeaveMember = {
	id: string;
	name: string | null;
	email: string | null;
};

export type ApprovedLeaveRow = {
	id: string;
	member_id: string;
	kind: string;
	days: number;
	status: 'approved';
	starts_at: string;
	ends_at: string;
};

export async function approvedLeaveCalendarEvents(
	caller: SupabaseClient,
	startDate: Date,
	endDate: Date,
	members: Map<string, CalendarLeaveMember>,
	timeZone: string,
	locale: Locale = 'ko'
): Promise<CalendarEvent[]> {
	const leave = await caller
		.from('leave')
		.select('id, member_id, kind, days, status, starts_at, ends_at')
		.eq('status', 'approved')
		.lt('starts_at', endDate.toISOString())
		.gte('ends_at', startDate.toISOString())
		.order('starts_at')
		.returns<ApprovedLeaveRow[]>();
	if (leave.error) throw new Error(`Failed to load approved leave calendar events: ${leave.error.message}`);
	return leave.data.map((row) => calendarEventFromApprovedLeave(row, members, timeZone, locale));
}

export function calendarEventFromApprovedLeave(
	leave: ApprovedLeaveRow,
	members: Map<string, CalendarLeaveMember>,
	timeZone: string,
	locale: Locale = 'ko'
): CalendarEvent {
	const member = members.get(leave.member_id);
	const email = member?.email ?? '';
	const name = member?.name || email.split('@')[0] || (locale === 'ko' ? '구성원' : 'Member');
	const id = `leave:${leave.id}`;
	const isAllDay = leave.days > 0.5;
	return {
		id,
		uid: id,
		title: `${name} · ${leaveKindLabel(leave.kind, leave.days, locale)}`,
		description: '',
		location: '',
		startISO: leave.starts_at,
		endISO: isAllDay ? lastCoveredMoment(leave.ends_at) : leave.ends_at,
		timeZone,
		isAllDay,
		color: dayOffColor,
		participants: [{ personID: leave.member_id, name, ...(email ? { email } : {}) }],
		createdByEmail: email,
		createdByName: name,
		updatedAt: leave.starts_at,
		readOnly: true,
		source: 'leave'
	};
}

function leaveKindLabel(kind: string, days: number, locale: Locale): string {
	if (kind !== 'leave' && kind !== '연차' && kind !== '반차') return kind;
	return localizedLeaveUnitName(days, locale);
}

function lastCoveredMoment(endsAt: string): string {
	return new Date(new Date(endsAt).getTime() - 1).toISOString();
}
