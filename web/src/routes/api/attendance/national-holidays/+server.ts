import { env } from '$env/dynamic/private';
import { asMember } from '$lib/server/control-plane';
import {
	nagerDateProvider,
	yearsBetween,
	type NationalHolidayProvider
} from '$lib/server/national-holidays';
import { error, json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

const provider: NationalHolidayProvider = nagerDateProvider();

export const GET: RequestHandler = async ({ request, platform, url }) => {
	const environment = { ...env, ...((platform?.env ?? {}) as Record<string, string | undefined>) };
	const projectURL = environment.SUPABASE_URL ?? '';
	const publishableKey = environment.SUPABASE_PUBLISHABLE_KEY ?? '';
	if (!projectURL || !publishableKey) error(500, 'the central plane is not configured');

	const authorization = request.headers.get('authorization') ?? '';
	const accessToken = authorization.startsWith('Bearer ') ? authorization.slice('Bearer '.length) : '';
	if (!accessToken) error(401, 'sign in first');

	const from = url.searchParams.get('from') ?? '';
	const to = url.searchParams.get('to') ?? '';
	let years: number[];
	try {
		years = yearsBetween(from, to);
	} catch (cause) {
		error(400, cause instanceof Error ? cause.message : 'the requested range is invalid');
	}

	const client = asMember({ projectURL, publishableKey }, accessToken);
	const company = await client.from('company').select('country').limit(1).maybeSingle<{ country: string }>();
	if (company.error) error(500, company.error.message);
	if (!company.data) error(403, 'you belong to no company');
	const country = company.data.country;

	try {
		const byYear = await Promise.all(years.map((year) => provider.holidayDates(country, year)));
		const dates = [...new Set(byYear.flat())].filter((date) => date >= from && date < to).sort();
		return json({ dates });
	} catch (cause) {
		error(502, cause instanceof Error ? cause.message : 'the holiday source is unavailable');
	}
};
