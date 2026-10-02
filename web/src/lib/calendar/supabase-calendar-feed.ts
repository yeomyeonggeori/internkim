import { supabase } from '$lib/supabase';
import type { CalendarSyncResponse } from '../../routes/calendar/calendar-layout-types';

async function signedInHeaders(): Promise<Record<string, string>> {
	const { data } = await supabase().auth.getSession();
	const accessToken = data.session?.access_token;
	if (!accessToken) throw new Error('sign in first');
	return { Authorization: `Bearer ${accessToken}`, 'Content-Type': 'application/json' };
}

async function addressOf(response: Response): Promise<string> {
	const written = await response.text();
	if (!response.ok) {
		let spoken = '';
		try {
			const parsed: unknown = JSON.parse(written);
			if (typeof parsed === 'object' && parsed !== null) {
				const { error, message } = parsed as { error?: unknown; message?: unknown };
				spoken = typeof error === 'string' ? error : typeof message === 'string' ? message : '';
			}
		} catch {
			spoken = written.trim();
		}
		throw new Error(spoken || `the calendar subscription returned ${response.status}`);
	}
	return (JSON.parse(written) as { address: string }).address;
}

export async function issueSupabaseCalendarSubscription(): Promise<CalendarSyncResponse> {
	const response = await fetch('/api/calendar/subscription', {
		method: 'POST',
		headers: await signedInHeaders()
	});
	return { icsURL: await addressOf(response) };
}

export const rotateSupabaseCalendarSubscription = issueSupabaseCalendarSubscription;
