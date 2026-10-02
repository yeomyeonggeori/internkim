import {
	issueSupabaseCalendarSubscription,
	rotateSupabaseCalendarSubscription
} from '$lib/calendar/supabase-calendar-feed';
import type { CalendarSyncResponse } from './calendar-layout-types';

export async function issueCalendarSubscriptionURL(): Promise<CalendarSyncResponse | null> {
	return issueSupabaseCalendarSubscription();
}

export async function rotateCalendarSubscriptionURL(): Promise<CalendarSyncResponse> {
	return rotateSupabaseCalendarSubscription();
}
