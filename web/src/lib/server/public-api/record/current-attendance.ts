import type { SupabaseClient } from '@supabase/supabase-js';
import { currentAttendanceSchema } from '$lib/attendance/current-attendance';
import { refusalOfPostgresError } from '../refusal';

export async function currentAttendance(caller: SupabaseClient) {
	const { data, error } = await caller.rpc('attendance_current');
	if (error) throw refusalOfPostgresError(error);
	return currentAttendanceSchema.parse(data);
}
