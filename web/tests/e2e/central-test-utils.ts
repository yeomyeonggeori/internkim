import { createClient, type SupabaseClient } from '@supabase/supabase-js';

export const exampleCompanyID = '000000cc-0000-0000-0000-000000000001';
export const member1ID = '000000ee-0000-0000-0000-000000000001';
export const member2ID = '000000ee-0000-0000-0000-000000000002';
export const member3ID = '000000ee-0000-0000-0000-000000000003';

export const member1Email = 'member1@example.com';
export const member2Email = 'member2@example.com';
export const member3Email = 'member3@example.com';

export const member1Name = '이샘플';
export const member2Name = '김예시';
export const member3Name = '박예시';

export type CentralPlaneLeaveStatus = 'requested' | 'approved' | 'rejected';

export type CentralPlaneLeave = {
	memberID: string;
	kind: string;
	days: number;
	status: CentralPlaneLeaveStatus;
	startISO: string;
	endISO: string;
	note?: string;
	isPaid?: boolean;
};

export function centralPlaneAdminClient(): SupabaseClient {
	const projectURL = process.env.SUPABASE_URL;
	const secretKey = process.env.SUPABASE_SECRET_KEY;
	if (!projectURL || !secretKey) {
		throw new Error('SUPABASE_URL and SUPABASE_SECRET_KEY must be set to seed central plane fixtures');
	}
	return createClient(projectURL, secretKey);
}

export async function seedLeave(leave: CentralPlaneLeave[]): Promise<string[]> {
	if (leave.length === 0) return [];
	const admin = centralPlaneAdminClient();
	const rows = leave.map((entry) => ({
		member_id: entry.memberID,
		kind: entry.kind,
		days: entry.days,
		is_paid: entry.isPaid ?? true,
		status: entry.status,
		starts_at: entry.startISO,
		ends_at: entry.endISO,
		note: entry.note ?? ''
	}));
	const inserted = await admin.from('leave').insert(rows).select('id');
	if (inserted.error) throw new Error(`Failed to seed leave: ${inserted.error.message}`);
	return inserted.data.map((row) => row.id as string);
}

export async function cleanupLeave(leaveIDs: string[]): Promise<void> {
	if (leaveIDs.length === 0) return;
	const admin = centralPlaneAdminClient();
	const deleted = await admin.from('leave').delete().in('id', leaveIDs);
	if (deleted.error) throw new Error(`Failed to clean up leave: ${deleted.error.message}`);
}
