import { env } from '$env/dynamic/private';
import { slugShape } from '$lib/company-path';
import { asMember, claimMemberFor, controlPlane, foundCompany } from '$lib/server/control-plane';
import { error, json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { memberAccessTokenOf } from '$lib/server/member-request';
import { setUpNotificationsFor } from '$lib/server/notifications-at-founding';

export const GET: RequestHandler = async ({ platform, url }) => {
	const environment = { ...env, ...((platform?.env ?? {}) as Record<string, string | undefined>) };
	const projectURL = environment.SUPABASE_URL ?? '';
	const serviceRoleKey = environment.SUPABASE_SECRET_KEY ?? environment.SUPABASE_SERVICE_ROLE_KEY ?? '';
	if (!projectURL || !serviceRoleKey) error(500, 'the central plane is not configured');

	const slug = (url.searchParams.get('slug') ?? '').trim().toLowerCase();
	if (!slugShape.test(slug)) return json({ slug, taken: false, usable: false });

	const client = controlPlane({ projectURL, serviceRoleKey });
	const { data, error: readError } = await client.from('company').select('name').eq('slug', slug).maybeSingle();
	if (readError) error(500, readError.message);
	return json({ slug, taken: Boolean(data), usable: !data, name: data?.name ?? null });
};

export const POST: RequestHandler = async ({ request, platform }) => {
	const environment = { ...env, ...((platform?.env ?? {}) as Record<string, string | undefined>) };
	const projectURL = environment.SUPABASE_URL ?? '';
	const publishableKey = environment.SUPABASE_PUBLISHABLE_KEY ?? '';
	const serviceRoleKey = environment.SUPABASE_SECRET_KEY ?? environment.SUPABASE_SERVICE_ROLE_KEY ?? '';
	if (!projectURL || !publishableKey || !serviceRoleKey) error(500, 'the central plane is not configured');

	const { accessToken } = await memberAccessTokenOf(request, { projectURL, serviceRoleKey });

	const { data: account } = await asMember({ projectURL, publishableKey }, accessToken).auth.getUser();
	const email = account.user?.email?.trim().toLowerCase();
	if (!account.user || !email) error(401, 'sign in first');

	const body = (await request.json().catch(() => ({}))) as {
		name?: unknown;
		slug?: unknown;
		timezone?: unknown;
		country?: unknown;
		locale?: unknown;
		invited?: unknown;
	};
	const name = typeof body.name === 'string' ? body.name.trim() : '';
	const slug = typeof body.slug === 'string' ? body.slug.trim().toLowerCase() : '';
	if (!name) error(400, 'a company needs a name');
	if (!slugShape.test(slug)) error(400, 'that address will not do');

	const client = controlPlane({ projectURL, serviceRoleKey });

	const already = await claimMemberFor(client, account.user.id, email);
	if (already) error(409, 'this account already belongs to a company');

	const invited = Array.isArray(body.invited)
		? [...new Set(body.invited.filter((entry): entry is string => typeof entry === 'string')
			.map((entry) => entry.trim().toLowerCase())
			.filter((entry) => entry.includes('@')))]
		: [];

	const founded = await foundCompany(
		client,
		{ accountID: account.user.id, email },
		{
			name,
			slug,
			country: typeof body.country === 'string' && body.country ? body.country : 'KR',
			locale: typeof body.locale === 'string' && body.locale ? body.locale : 'ko',
			timezone: typeof body.timezone === 'string' && body.timezone ? body.timezone : 'Asia/Seoul'
		},
		invited
	);

	const notifications = await setUpNotificationsFor(environment, accessToken, email);

	return json({ ...founded, slug, notifications });
};
