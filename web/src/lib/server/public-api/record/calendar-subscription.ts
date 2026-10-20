import { companyHasAFeedToken, feedAddressOf, forgetFeedToken, issueFeedToken } from '$lib/server/calendar/feed-token';
import type { RecordContext, ToolPlace } from './company';

export async function calendarSubscriptionGet(context: RecordContext) {
	return { registered: await companyHasAFeedToken(context.caller, context.companyID) };
}

export async function calendarSubscriptionAdd(context: RecordContext, place: ToolPlace) {
	return { address: feedAddressOf(place.appAddress, await issueFeedToken(context.caller)) };
}

export async function calendarSubscriptionDelete(context: RecordContext) {
	await forgetFeedToken(context.caller);
	return { deleted: true };
}
