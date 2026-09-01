import { supabase } from '$lib/supabase';

export async function announceApprovalRequest(approvalID: string): Promise<void> {
	const { data } = await supabase().auth.getSession();
	const accessToken = data.session?.access_token;
	if (!accessToken) return;
	const response = await fetch('/api/approval/announce', {
		method: 'POST',
		headers: { Authorization: `Bearer ${accessToken}`, 'Content-Type': 'application/json' },
		body: JSON.stringify({ approvalID })
	});
	if (!response.ok) {
		throw new Error(`approval announcement answered ${response.status}`);
	}
}

export function announceApprovalRequestInBackground(approvalID: string): void {
	void announceApprovalRequest(approvalID).catch((failure: unknown) => {
		console.warn('the administrators were not told about approval', approvalID, failure);
	});
}
