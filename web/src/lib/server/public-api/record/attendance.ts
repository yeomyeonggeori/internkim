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

export function timeOfInstant(timezone: string, instant: string): string {
	return new Intl.DateTimeFormat('en-GB', {
		timeZone: timezone,
		hour12: false,
		hour: '2-digit',
		minute: '2-digit'
	}).format(new Date(instant));
}

export async function attendanceOfCompany(
	caller: SupabaseClient,
	from: string,
	to: string
): Promise<AttendanceRow[]> {
	const { data, error } = await caller
		.from('attendance')
		.select('id, member_id, kind, location, occurred_at, original_occurred_at, edit_reason')
		.gte('occurred_at', from)
		.lte('occurred_at', to)
		.order('occurred_at')
		.returns<AttendanceRow[]>();
	if (error) throw new Error(error.message);
	return data ?? [];
}
