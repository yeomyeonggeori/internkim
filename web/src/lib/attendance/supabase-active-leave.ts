import { supabase } from '$lib/supabase';
import { supabaseLeaveTypeDirectory } from './supabase-leave-types';
import type { AttendanceActiveLeave } from '../../routes/attendance/attendance-context.svelte';

type CoveringLeaveRow = {
	id: string;
	kind: string;
	days: number;
	starts_at: string;
	ends_at: string;
};

export async function supabaseActiveLeave(
	memberID: string,
	timeZone: string,
	now: Date
): Promise<AttendanceActiveLeave | undefined> {
	const covering = await supabase()
		.from('leave')
		.select('id, kind, days, starts_at, ends_at')
		.eq('member_id', memberID)
		.eq('status', 'approved')
		.lte('starts_at', now.toISOString())
		.gt('ends_at', now.toISOString())
		.order('starts_at')
		.limit(1)
		.returns<CoveringLeaveRow[]>();
	if (covering.error) throw new Error(covering.error.message);
	const row = covering.data[0];
	if (!row) return undefined;

	const directory = await supabaseLeaveTypeDirectory();
	return {
		requestID: row.id,
		occurrenceID: row.id,
		leaveTypeID: row.kind,
		leaveTypeName: directory.nameOf(row.kind),
		startTime: timeIn(new Date(row.starts_at), timeZone),
		endTime: timeIn(new Date(row.ends_at), timeZone),
		deductionMilliDays: Math.round(row.days * 1000),
		startAt: row.starts_at,
		endAt: row.ends_at
	};
}

export async function returnEarlyFromSupabaseLeave(locationID?: string): Promise<void> {
	const { error } = await supabase().rpc('leave_return_early', {
		work_location: locationID ?? ''
	});
	if (error) throw new Error(error.message);
}

function timeIn(instant: Date, timeZone: string): string {
	return new Intl.DateTimeFormat('en-GB', {
		timeZone,
		hour: '2-digit',
		minute: '2-digit',
		hour12: false
	}).format(instant);
}
