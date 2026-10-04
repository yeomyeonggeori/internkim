import type { SupabaseClient } from '@supabase/supabase-js';
import { currentAttendanceSchema } from '$lib/attendance/current-attendance';
import { RecordRefusedTheWrite, statusOfPostgresCode } from './tasks';

export async function currentAttendance(caller: SupabaseClient) {
	const { data, error } = await caller.rpc('attendance_current');
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));
	return currentAttendanceSchema.parse(data);
}
