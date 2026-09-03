import type {
	LeaveApprovalDecision,
	LeaveApprovalInbox,
	LeaveApprovalRequest
} from './leave-approval-types';
import { decideSupabaseLeave, supabaseLeaveApprovalInbox } from '$lib/attendance/supabase-leave';

export function fetchLeaveApprovalInbox(): Promise<LeaveApprovalInbox> {
	return supabaseLeaveApprovalInbox();
}

export function decideLeaveApproval(
	requestID: string,
	decision: LeaveApprovalDecision
): Promise<LeaveApprovalRequest> {
	return decideSupabaseLeave(requestID, decision);
}
