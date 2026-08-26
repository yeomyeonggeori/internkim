import { supabase } from '$lib/supabase';

export type SelfTestOutcome = {
	reached: number;
	pruned: number;
};

export async function sendTestNotification(title: string, body: string): Promise<SelfTestOutcome> {
	const { data } = await supabase().auth.getSession();
	const accessToken = data.session?.access_token;
	if (!accessToken) throw new Error('sign in first');

	const response = await fetch('/api/notifications/test', {
		method: 'POST',
		headers: { Authorization: `Bearer ${accessToken}`, 'Content-Type': 'application/json' },
		body: JSON.stringify({ title, body })
	});
	if (!response.ok) throw new Error((await response.text()).trim() || `the request returned ${response.status}`);
	return (await response.json()) as SelfTestOutcome;
}
