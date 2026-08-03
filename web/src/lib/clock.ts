import { supabase } from './supabase';

export type ClockKind = 'clock_in' | 'clock_out';

export type Member = {
	id: string;
	company_id: string;
	email: string | null;
	timezone: string | null;
};

export type Company = {
	name: string;
	timezone: string;
	work_locations: string[] | null;
};

export type AttendanceEntry = {
	id: string;
	kind: ClockKind;
	location: string | null;
	occurred_at: string;
};

export async function fetchMember(): Promise<Member | null> {
	const { data, error } = await supabase()
		.from('member')
		.select('id, company_id, email, timezone')
		.maybeSingle();
	if (error) throw new Error(error.message);
	return data;
}

export async function fetchCompany(): Promise<Company | null> {
	const { data, error } = await supabase()
		.from('company')
		.select('name, timezone, work_locations')
		.maybeSingle();
	if (error) throw new Error(error.message);
	return data;
}

export async function fetchMyAttendance(memberID: string, limit = 20): Promise<AttendanceEntry[]> {
	const { data, error } = await supabase()
		.from('attendance')
		.select('id, kind, location, occurred_at')
		.eq('member_id', memberID)
		.order('occurred_at', { ascending: false })
		.limit(limit);
	if (error) throw new Error(error.message);
	return data ?? [];
}

export async function recordAttendance(
	memberID: string,
	kind: ClockKind,
	location: string | null,
	occurredAt?: Date,
): Promise<void> {
	const { error } = await supabase()
		.from('attendance')
		.insert({
			member_id: memberID,
			kind,
			location: kind === 'clock_in' ? location : null,
			...(occurredAt ? { occurred_at: occurredAt.toISOString() } : {}),
		});
	if (error) throw new Error(error.message);
}

export function nextClockKind(entries: AttendanceEntry[]): ClockKind {
	return entries[0]?.kind === 'clock_in' ? 'clock_out' : 'clock_in';
}

export function dayInTimezone(instant: string, timezone: string | null): string {
	return new Date(instant).toLocaleDateString('sv-SE', timezone ? { timeZone: timezone } : {});
}

// Someone who forgets to clock out and clocks in the next day would otherwise be
// stuck: the record rejects a second clock-in. Rather than invent a leaving time,
// the client asks them when they actually left.
export function forgottenClockOut(
	entries: AttendanceEntry[],
	timezone: string | null,
	now: Date = new Date(),
): AttendanceEntry | null {
	const latest = entries[0];
	if (!latest || latest.kind !== 'clock_in') return null;
	const today = dayInTimezone(now.toISOString(), timezone);
	return dayInTimezone(latest.occurred_at, timezone) < today ? latest : null;
}

export function isPlausibleClockOut(openedAt: string, leftAt: Date, now: Date = new Date()): boolean {
	return leftAt > new Date(openedAt) && leftAt <= now;
}

// A datetime-local field means a wall clock, and the wall clock that matters is
// the member's, not the browser's. Reading it as browser-local shifts the record
// for anyone working in another zone.
export function instantFromLocalInput(value: string, timezone: string | null): Date {
	const naive = new Date(`${value}:00Z`);
	if (!timezone || Number.isNaN(naive.getTime())) return new Date(value);
	const offset = naive.getTime() - new Date(localInputOf(naive, timezone) + ':00Z').getTime();
	return new Date(naive.getTime() + offset);
}

export function localInputOf(instant: Date, timezone: string | null): string {
	const formatted = instant.toLocaleString('sv-SE', timezone ? { timeZone: timezone } : {});
	return formatted.replace(' ', 'T').slice(0, 16);
}
