import type { SupabaseClient } from '@supabase/supabase-js';

export type AttendanceRow = {
	id: string;
	member_id: string;
	kind: string;
	location: string | null;
	occurred_at: string;
	original_occurred_at: string | null;
	edit_reason: string | null;
};

export class NoSuchAttendanceRecord extends Error {
	constructor(
		readonly hint: string,
		readonly candidates: string[]
	) {
		super(
			candidates.length === 0
				? `no attendance record here goes by ${hint}`
				: `${hint} could be ${candidates.join(', ')}; name one of them exactly`
		);
		this.name = 'NoSuchAttendanceRecord';
	}
}

export function attendanceWrittenByHand(rows: AttendanceRow[]): AttendanceRow[] {
	return rows.filter((row) => row.edit_reason !== null || row.original_occurred_at !== null);
}

export function timeOfInstant(timezone: string, instant: string): string {
	return new Intl.DateTimeFormat('en-GB', {
		timeZone: timezone,
		hour12: false,
		hour: '2-digit',
		minute: '2-digit'
	}).format(new Date(instant));
}

const heldColumns = 'id, member_id, kind, location, occurred_at, original_occurred_at, edit_reason';

// The newest rows are the ones a hint refers to and the ones a month view
// shows, and a company of any age has more than a page of them, so the read
// takes them from the recent end and hands them back in reading order.
export async function attendanceOfCompany(
	caller: SupabaseClient,
	from: string,
	to: string,
	mostRecent: number,
	memberIDs?: string[]
): Promise<AttendanceRow[]> {
	let query = caller
		.from('attendance')
		.select(heldColumns)
		.gte('occurred_at', from)
		.lte('occurred_at', to);
	if (memberIDs) query = query.in('member_id', memberIDs);
	const { data, error } = await query
		.order('occurred_at', { ascending: false })
		.limit(mostRecent)
		.returns<AttendanceRow[]>();
	if (error) throw new Error(error.message);
	return (data ?? []).slice().reverse();
}

export async function attendanceClock(caller: SupabaseClient): Promise<string> {
	const { data, error } = await caller.rpc('attendance_server_time');
	if (error) throw new Error(error.message);
	const moment = new Date(String(data));
	if (Number.isNaN(moment.getTime())) throw new Error(`the record's clock answered ${String(data)}`);
	return moment.toISOString();
}

export async function attendanceBackdatedAfterMinutes(caller: SupabaseClient): Promise<number> {
	const { data, error } = await caller.rpc('attendance_backdated_after_minutes');
	if (error) throw new Error(error.message);
	const minutes = Number(data);
	if (!Number.isInteger(minutes) || minutes <= 0) {
		throw new Error('the backdated attendance threshold must be a positive integer');
	}
	return minutes;
}

export async function attendanceByID(
	caller: SupabaseClient,
	eventID: string
): Promise<AttendanceRow | null> {
	const { data, error } = await caller
		.from('attendance')
		.select(heldColumns)
		.eq('id', eventID)
		.maybeSingle<AttendanceRow>();
	if (error) throw new Error(error.message);
	return data;
}
