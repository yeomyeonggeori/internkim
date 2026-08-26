import { supabase } from '$lib/supabase';

export async function announceTaskMoved(taskID: string): Promise<void> {
	const { data } = await supabase().auth.getSession();
	const accessToken = data.session?.access_token;
	if (!accessToken) return;
	await fetch('/api/flow/announce', {
		method: 'POST',
		headers: { Authorization: `Bearer ${accessToken}`, 'Content-Type': 'application/json' },
		body: JSON.stringify({ taskID })
	}).catch(() => undefined);
}
