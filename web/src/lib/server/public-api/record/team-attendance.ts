import type { SupabaseClient } from '@supabase/supabase-js';
import { attendanceTeamPageInputSchema, attendanceTeamPageSchema } from '$lib/attendance/team-page';
import { RecordRefusedTheWrite, statusOfPostgresCode } from './tasks';

export async function teamAttendance(caller: SupabaseClient, untrustedInput: Record<string, unknown>) {
	const input = attendanceTeamPageInputSchema.parse(untrustedInput);
	const { data, error } = await caller.rpc('attendance_team_page', {
		page_kind: input.pageKind,
		team_offset: input.teamOffset ?? 0,
		team_limit: input.teamLimit ?? 12,
		selected_team_key: input.selectedTeamKey ?? null,
		member_offset: input.memberOffset ?? 0,
		member_limit: input.memberLimit ?? 24,
		search_text: input.searchText ?? '',
		location_filter: input.locationFilter ?? ''
	});
	if (error) throw new RecordRefusedTheWrite(error.message, statusOfPostgresCode(error.code));
	return attendanceTeamPageSchema.parse(data);
}

export async function legacyTeamAttendance(caller: SupabaseClient, input: Record<string, unknown>) {
 const answer=await teamAttendance(caller,input);
 return attendanceTeamPageSchema.omit({companyName:true,companySummary:true}).strip().parse(answer);
}
