import {
	asMember,
	controlPlane,
	sessionForMember,
	type ControlPlaneCredentials
} from '$lib/server/control-plane';
import { companyCalendarEntries } from '$lib/calendar/company-calendar';
import { calendarFeedOf } from './ics';
import { companyOfFeedToken } from './feed-token';

const daysBehind = 90;
const daysAhead = 400;

export type FeedCredentials = ControlPlaneCredentials & { publishableKey: string };

function windowAround(now: Date): { from: Date; to: Date } {
	const day = 24 * 60 * 60 * 1000;
	return {
		from: new Date(now.getTime() - daysBehind * day),
		to: new Date(now.getTime() + daysAhead * day)
	};
}

async function anAdminOf(credentials: ControlPlaneCredentials, companyID: string): Promise<string | null> {
	const { data, error } = await controlPlane(credentials)
		.from('member')
		.select('id')
		.eq('company_id', companyID)
		.eq('is_admin', true)
		.eq('status', 'active')
		.order('joined_at')
		.limit(1)
		.maybeSingle<{ id: string }>();
	if (error) throw new Error(`calendar subscription: ${error.message}`);
	return data?.id ?? null;
}

export async function calendarFeedForToken(
	credentials: FeedCredentials,
	token: string,
	now: Date
): Promise<string | null> {
	const companyID = await companyOfFeedToken(credentials, token);
	if (!companyID) return null;

	const readerID = await anAdminOf(credentials, companyID);
	if (!readerID) return null;

	const session = await sessionForMember(credentials, readerID);
	const caller = asMember(
		{ projectURL: credentials.projectURL, publishableKey: credentials.publishableKey },
		session.accessToken
	);

	const company = await caller
		.from('company')
		.select('name, timezone')
		.eq('id', companyID)
		.maybeSingle<{ name: string | null; timezone: string | null }>();
	if (company.error) throw new Error(`calendar subscription: ${company.error.message}`);

	const timezone = company.data?.timezone?.trim() || 'Asia/Seoul';
	const window = windowAround(now);
	const entries = await companyCalendarEntries(caller, window.from, window.to, timezone);

	return calendarFeedOf(entries, company.data?.name?.trim() || 'internkim', timezone);
}
