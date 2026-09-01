import { supabase } from '$lib/supabase';

export async function announceToTheCompany(what: 'clock' | 'leave'): Promise<void> {
	const { data } = await supabase().auth.getSession();
	if (!data.session) return;
	await supabase()
		.functions.invoke('announce-attendance', { body: { what } })
		.catch(() => undefined);
}
