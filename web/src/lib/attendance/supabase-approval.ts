import { supabase } from '$lib/supabase';
import {
	attendanceApprovalRequestFrom,
	type AttendanceApprovalDecision,
	type AttendanceApprovalRequest,
	type AttendanceApprovalRow
} from '../../routes/attendance/approval/attendance-approval-types';

export async function supabasePendingApprovals(): Promise<AttendanceApprovalRequest[]> {
	const { data, error } = await supabase().rpc('approval_pending');
	if (error) throw new Error(error.message);
	return ((data ?? []) as AttendanceApprovalRow[]).map(attendanceApprovalRequestFrom);
}

export async function decideSupabaseApproval(
	approvalID: string,
	decision: AttendanceApprovalDecision,
	note: string
): Promise<void> {
	const { error } = await supabase().rpc('approval_decide', {
		approval_id: approvalID,
		decision,
		note
	});
	if (error) throw new Error(error.message);
}

export async function withdrawSupabaseApproval(approvalID: string): Promise<void> {
	const { error } = await supabase().rpc('approval_withdraw', { approval_id: approvalID });
	if (error) throw new Error(error.message);
}
