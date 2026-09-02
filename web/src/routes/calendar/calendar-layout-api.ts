import {
	issueSupabaseCalendarSubscription,
	rotateSupabaseCalendarSubscription,
	supabaseCalendarSubscription
} from '$lib/calendar/supabase-calendar-feed';
import type { CalendarSyncResponse } from './calendar-layout-types';

export async function fetchCalendarSyncInformation(): Promise<CalendarSyncResponse | null> {
	return supabaseCalendarSubscription();
}

export async function issueCalendarSubscriptionURL(): Promise<CalendarSyncResponse | null> {
	return issueSupabaseCalendarSubscription();
}

export async function rotateCalendarSubscriptionURL(): Promise<CalendarSyncResponse> {
	return rotateSupabaseCalendarSubscription();
}
