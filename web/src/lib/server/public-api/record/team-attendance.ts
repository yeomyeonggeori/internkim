import type { SupabaseClient } from '@supabase/supabase-js';
import type { z } from 'zod';
import { attendanceTeamPageSchema, type attendanceTeamPageInputSchema } from '$lib/attendance/team-page';
import { refusalOfPostgresError } from '../refusal';

type TeamAttendanceInput = z.infer<typeof attendanceTeamPageInputSchema>;

export async function teamAttendance(caller: SupabaseClient, input: TeamAttendanceInput) {
	const { data, error } = await caller.rpc('attendance_team_page', {
		page_kind: input.pageKind,
		team_offset: input.teamOffset ?? 0,
		team_limit: input.teamLimit ?? 12,
		selected_team_key: input.selectedTeamKey ?? null,
		member_offset: input.memberOffset ?? 0,
		member_limit: input.memberLimit ?? 24,
		search_text: input.searchText ?? '',
		location_filter: input.locationFilter ?? '',
		status_filter: input.statusFilter ?? ''
	});
	if (error) throw refusalOfPostgresError(error);
	return attendanceTeamPageSchema.parse(data);
}

export async function legacyTeamAttendance(caller: SupabaseClient, input: TeamAttendanceInput) {
	const answer = await teamAttendance(caller, input);
	return attendanceTeamPageSchema.omit({ companyName: true, companySummary: true }).strip().parse(answer);
}
