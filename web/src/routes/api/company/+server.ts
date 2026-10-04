import { env } from '$env/dynamic/private';
import { isUsableCompanyAddress } from '$lib/company-path';
import {
	asMember,
	claimMemberFor,
	CompanyAddressTaken,
	controlPlane,
	foundCompany,
	planeCredentialsOf
} from '$lib/server/control-plane';
import { error, json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { memberAccessTokenOf } from '$lib/server/member-request';
import { setUpNotificationsFor } from '$lib/server/notifications-at-founding';

export const GET: RequestHandler = async ({ platform, url }) => {
	const environment = { ...env, ...((platform?.env ?? {}) as Record<string, string | undefined>) };
	const projectURL = environment.SUPABASE_URL ?? '';
	const serviceRoleKey = environment.SUPABASE_SECRET_KEY ?? '';
	if (!projectURL || !serviceRoleKey) error(500, 'the central plane is not configured');

	const slug = (url.searchParams.get('slug') ?? '').trim().toLowerCase();
	if (!isUsableCompanyAddress(slug)) return json({ slug, taken: false, usable: false });

	const client = controlPlane({ projectURL, serviceRoleKey });
	const { data, error: readError } = await client.from('company').select('id').eq('slug', slug).maybeSingle();
	if (readError) error(500, readError.message);
	return json({ slug, taken: Boolean(data), usable: !data });
};

export const POST: RequestHandler = async ({ request, platform }) => {
	const environment = { ...env, ...((platform?.env ?? {}) as Record<string, string | undefined>) };
	const plane = planeCredentialsOf(environment);
	if (!plane) error(500, 'the central plane is not configured');

	const { accessToken } = await memberAccessTokenOf(request, plane);

	const { data: account } = await asMember(plane, accessToken).auth.getUser();
	const email = account.user?.email?.trim().toLowerCase();
	if (!account.user || !email) error(401, 'sign in first');

	const body = (await request.json().catch(() => ({}))) as {
		name?: unknown;
		slug?: unknown;
		founderName?: unknown;
		timezone?: unknown;
		locale?: unknown;
	};
	const name = typeof body.name === 'string' ? body.name.trim() : '';
	const slug = typeof body.slug === 'string' ? body.slug.trim().toLowerCase() : '';
	const founderName = typeof body.founderName === 'string' ? body.founderName.trim() : '';
	if (!name) error(400, 'a company needs a name');
	if (!founderName) error(400, 'the founder needs a name');
	if (!isUsableCompanyAddress(slug)) error(400, 'that address will not do');

	const client = controlPlane(plane);

	const already = await claimMemberFor(client, account.user.id, email);
	if (already) error(409, 'this account already belongs to a company');

	const founded = await companyFoundedUnlessTheAddressIsTaken(
		client,
		{ accountID: account.user.id, email, name: founderName },
		{
			name,
			slug,
			country: 'KR',
			locale: body.locale === 'en' ? 'en' : 'ko',
			timezone: isTimeZone(body.timezone) ? body.timezone : 'Asia/Seoul'
		}
	);

	const notifications = await setUpNotificationsFor(environment, accessToken, email, client);

	return json({ ...founded, slug, notifications });
};

async function companyFoundedUnlessTheAddressIsTaken(
	...founding: Parameters<typeof foundCompany>
): ReturnType<typeof foundCompany> {
	try {
		return await foundCompany(...founding);
	} catch (refusal) {
		if (refusal instanceof CompanyAddressTaken) error(409, `${refusal.slug} is already a company's address`);
		throw refusal;
	}
}

function isTimeZone(value: unknown): value is string {
	if (typeof value !== 'string' || !value) return false;
	try {
		new Intl.DateTimeFormat('en', { timeZone: value });
		return true;
	} catch {
		return false;
	}
}
