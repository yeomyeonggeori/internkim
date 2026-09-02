import { env } from '$env/dynamic/private';
import type { CalendarHolidayLocale } from '$lib/calendar/holiday';
import { asMember } from '$lib/server/control-plane';
import {
	companyCalendarHolidays,
	mergedCalendarHolidays,
	nationalCalendarHolidays,
	type CompanyHolidayRecord
} from '$lib/server/calendar/holidays';
import {
	nagerDateProvider,
	yearsBetween,
	type NationalHolidayProvider
} from '$lib/server/national-holidays';
import { error, json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

const provider: NationalHolidayProvider = nagerDateProvider();

type CompanyRow = {
	country: string;
	rules: { companyHolidays?: CompanyHolidayRecord[] } | null;
};

function localeOf(value: string | null): CalendarHolidayLocale {
	return value === 'en' ? 'en' : 'ko';
}

function accessTokenOf(request: Request): string {
	const authorization = request.headers.get('authorization') ?? '';
	if (!authorization.startsWith('Bearer ')) error(401, 'sign in first');
	return authorization.slice('Bearer '.length);
}

async function callersCompany(request: Request, platform: App.Platform | undefined): Promise<CompanyRow> {
	const environment = { ...env, ...((platform?.env ?? {}) as Record<string, string | undefined>) };
	const projectURL = environment.SUPABASE_URL ?? '';
	const publishableKey = environment.SUPABASE_PUBLISHABLE_KEY ?? '';
	if (!projectURL || !publishableKey) error(500, 'the central plane is not configured');

	const client = asMember({ projectURL, publishableKey }, accessTokenOf(request));
	const company = await client
		.from('company')
		.select('country, rules')
		.limit(1)
		.maybeSingle<CompanyRow>();
	if (company.error) error(500, company.error.message);
	if (!company.data) error(403, 'you belong to no company');
	return company.data;
}

export const GET: RequestHandler = async ({ request, platform, url }) => {
	const from = url.searchParams.get('from') ?? '';
	const to = url.searchParams.get('to') ?? '';
	const locale = localeOf(url.searchParams.get('locale'));
	try {
		yearsBetween(from, to);
	} catch (cause) {
		error(400, cause instanceof Error ? cause.message : 'the requested range is invalid');
	}

	const company = await callersCompany(request, platform);
	const companyHolidays = companyCalendarHolidays(company.rules?.companyHolidays ?? [], from, to);

	try {
		const national = await nationalCalendarHolidays(provider, company.country, locale, from, to);
		return json({ holidays: mergedCalendarHolidays(national, companyHolidays), degraded: false });
	} catch (cause) {
		return json({
			holidays: mergedCalendarHolidays([], companyHolidays),
			degraded: true,
			errorCode: 'holiday_source_unavailable',
			message: cause instanceof Error ? cause.message : 'the holiday source is unavailable'
		});
	}
};
