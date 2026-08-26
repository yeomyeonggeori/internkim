import { supabase } from '$lib/supabase';

export async function announceToTheCompany(what: 'clock' | 'leave'): Promise<void> {
	const { data } = await supabase().auth.getSession();
	const accessToken = data.session?.access_token;
	if (!accessToken) return;
	await fetch('/api/attendance/announce', {
		method: 'POST',
		headers: { Authorization: `Bearer ${accessToken}`, 'Content-Type': 'application/json' },
		body: JSON.stringify({ what })
	}).catch(() => undefined);
}
