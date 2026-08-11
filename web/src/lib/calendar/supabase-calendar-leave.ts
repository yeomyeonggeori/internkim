import {
	companyDateOfTimestamp,
	isCompanyAllDayRange
} from '../attendance/supabase-leave-range';
import { supabase } from '../supabase';
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
	startDate: Date,
	endDate: Date,
	members: Map<string, CalendarLeaveMember>,
	timeZone: string
): Promise<CalendarEvent[]> {
	const leave = await supabase()
		.from('leave')
		.select('id, member_id, kind, days, status, starts_at, ends_at')
		.eq('status', 'approved')
		.lt('starts_at', endDate.toISOString())
		.gte('ends_at', startDate.toISOString())
		.order('starts_at')
		.returns<ApprovedLeaveRow[]>();
	if (leave.error) throw new Error(`Failed to load approved leave calendar events: ${leave.error.message}`);
	return leave.data.map((row) => calendarEventFromApprovedLeave(row, members, timeZone));
}

export function calendarEventFromApprovedLeave(
	leave: ApprovedLeaveRow,
	members: Map<string, CalendarLeaveMember>,
	timeZone: string
): CalendarEvent {
	const member = members.get(leave.member_id);
	const email = member?.email ?? '';
	const name = member?.name || email.split('@')[0] || '구성원';
	const id = `leave:${leave.id}`;
	const isAllDay = isCompanyAllDayRange(leave.starts_at, leave.ends_at, timeZone);
	return {
		id,
		uid: id,
		title: `${name} · ${leaveKindLabel(leave.kind)}`,
		description: '',
		location: '',
		startISO: isAllDay ? calendarMidnightISO(leave.starts_at, timeZone) : leave.starts_at,
		endISO: isAllDay ? calendarMidnightISO(leave.ends_at, timeZone) : leave.ends_at,
		timeZone,
		isAllDay,
		color: '',
		participants: [{ personID: leave.member_id, name, ...(email ? { email } : {}) }],
		createdByEmail: email,
		createdByName: name,
		updatedAt: leave.starts_at,
		readOnly: true,
		source: 'leave'
	};
}

function leaveKindLabel(kind: string): string {
	return kind === 'leave' ? '휴가' : kind;
}

function calendarMidnightISO(value: string, timeZone: string): string {
	return `${companyDateOfTimestamp(value, timeZone)}T00:00:00.000Z`;
}
