import {
	asMember,
	controlPlane,
	sessionForMember,
	type ControlPlaneCredentials,
	type PlaneCredentials
} from '$lib/server/control-plane';
import { localeOf } from '$lib/i18n/locale';
import { calendarMembers, companyCalendarEntries } from '$lib/server/public-api/record/company-calendar';
import { calendarFeedOf } from './ics';
import { companyOfFeedToken } from './feed-token';

const daysBehind = 90;
const daysAhead = 400;

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
	credentials: PlaneCredentials,
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
		.select('name, timezone, locale')
		.eq('id', companyID)
		.maybeSingle<{ name: string | null; timezone: string | null; locale: string | null }>();
	if (company.error) throw new Error(`calendar subscription: ${company.error.message}`);

	const timezone = company.data?.timezone?.trim() || 'Asia/Seoul';
	const window = windowAround(now);
	const entries = await companyCalendarEntries(
		{
			caller,
			members: await calendarMembers(caller),
			timeZone: timezone,
			locale: localeOf(company.data?.locale)
		},
		window.from,
		window.to
	);

	return calendarFeedOf(entries, company.data?.name?.trim() || 'internkim', timezone);
}
