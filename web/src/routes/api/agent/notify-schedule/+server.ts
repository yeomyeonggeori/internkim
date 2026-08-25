import { error, json } from '@sveltejs/kit';
import { callingAgent, environmentOf } from '$lib/server/agent-request';
import { readNotificationSettings, readTimeOfDay } from '$lib/notifications/categories';
import type { RequestHandler } from './$types';

type MemberRow = { messenger: Record<string, string> | null; notification_settings: unknown };

// A device knows what is on somebody's calendar; the plane knows the hour they
// asked to hear about it. This is the plane answering "whose hour is it now",
// so the device does not have to hold a copy of everyone's settings.
export const GET: RequestHandler = async ({ request, url, platform }) => {
	const environment = environmentOf(platform);
	const { client, companyID } = await callingAgent(request, environment);

	const messenger = (url.searchParams.get('platform') ?? '').trim();
	if (!messenger) error(400, 'which messenger these people are on');
	const at = readTimeOfDay(url.searchParams.get('at'));
	if (!at) error(400, 'at must be a time of day, as HH:MM');

	const { data, error: refusal } = await client
		.from('member')
		.select('messenger, notification_settings')
		.eq('company_id', companyID)
		.eq('status', 'active')
		.returns<MemberRow[]>();
	if (refusal) error(500, refusal.message);

	const externalIDs = (data ?? [])
		.filter((member) => {
			const settings = readNotificationSettings(member.notification_settings);
			return settings.categories.calendar && settings.calendarAt === at;
		})
		.map((member) => member.messenger?.[messenger] ?? '')
		.filter((externalID) => externalID !== '');

	return json({ at, externalIDs });
};
