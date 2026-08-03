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
): Promise<void> {
	const { error } = await supabase()
		.from('attendance')
		.insert({ member_id: memberID, kind, location: kind === 'clock_in' ? location : null });
	if (error) throw new Error(error.message);
}

export function nextClockKind(entries: AttendanceEntry[]): ClockKind {
	return entries[0]?.kind === 'clock_in' ? 'clock_out' : 'clock_in';
}
