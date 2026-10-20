import { invokeTool } from '$lib/public-api-call';
import type { CalendarSyncResponse } from './calendar-layout-types';

export async function readCalendarSubscription(): Promise<CalendarSyncResponse> {
	const { registered } = await invokeTool<{ registered: boolean }>('calendar_subscription_get', {});
	return { isRegistered: registered };
}

export async function makeCalendarSubscriptionURL(): Promise<CalendarSyncResponse> {
	const { address } = await invokeTool<{ address: string }>('calendar_subscription_add', {});
	return { icsURL: address, isRegistered: true };
}

export async function revokeCalendarSubscriptionURL(): Promise<CalendarSyncResponse> {
	await invokeTool<{ deleted: boolean }>('calendar_subscription_delete', {});
	return { isRegistered: false };
}
