import { isSupabaseConfigured } from '$lib/supabase';
import {
	issueSupabaseCalendarSubscription,
	rotateSupabaseCalendarSubscription,
	supabaseCalendarSubscription
} from '$lib/calendar/supabase-calendar-feed';
import type { CalendarSyncResponse } from './calendar-layout-types';

export async function fetchCalendarSyncInformation(): Promise<CalendarSyncResponse | null> {
	if (isSupabaseConfigured()) return supabaseCalendarSubscription();
	return readCalendarSyncInformation();
}

export async function issueCalendarSubscriptionURL(): Promise<CalendarSyncResponse | null> {
	if (isSupabaseConfigured()) return issueSupabaseCalendarSubscription();
	return readCalendarSyncInformation();
}

async function readCalendarSyncInformation(): Promise<CalendarSyncResponse | null> {
	const response = await fetch('/calendar/api/sync', { credentials: 'include' });
	if (!response.ok) return null;
	return (await response.json()) as CalendarSyncResponse;
}

export async function rotateCalendarSubscriptionURL(errorMessage: string): Promise<CalendarSyncResponse> {
	if (isSupabaseConfigured()) return rotateSupabaseCalendarSubscription();
	const response = await fetch('/calendar/api/ics-token', {
		method: 'POST',
		credentials: 'include'
	});
	if (!response.ok) throw new Error(errorMessage);
	return (await response.json()) as CalendarSyncResponse;
}
