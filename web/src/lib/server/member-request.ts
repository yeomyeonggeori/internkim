import { error } from '@sveltejs/kit';
import type { SupabaseClient } from '@supabase/supabase-js';
import { fullPublicAPIPermission, type PublicAPIPermission } from '$lib/public-api-permission';
import {
	asMember,
	controlPlane,
	isPersonalAccessToken,
	sessionForPersonalAccessToken,
	TokenOwnerHasLeft,
	type ControlPlaneCredentials,
	type PersonalAccessTokenSession,
} from './control-plane';
import type { Environment } from './agent-request';

export type CallingMember = {
	accessToken: string;
	caller: SupabaseClient;
	record: SupabaseClient;
	memberID: string;
	companyID: string;
	email: string;
	permission: PublicAPIPermission;
	tokenName: string;
};

export type MemberCall = {
	accessToken: string;
	permission: PublicAPIPermission;
	tokenName: string;
};

export async function memberAccessTokenOf(
	request: Request,
	credentials: ControlPlaneCredentials,
): Promise<MemberCall> {
	const authorization = request.headers.get('authorization') ?? '';
	const presented = authorization.startsWith('Bearer ') ? authorization.slice('Bearer '.length) : '';
	if (!presented) error(401, 'sign in first');
	if (!isPersonalAccessToken(presented)) {
		return { accessToken: presented, permission: fullPublicAPIPermission, tokenName: '' };
	}

	const session = await sessionOfTokenOrRefusal(credentials, presented);
	if (!session) error(401, 'that key belongs to nobody');
	return { accessToken: session.accessToken, permission: session.permission, tokenName: session.tokenName };
}

async function sessionOfTokenOrRefusal(
	credentials: ControlPlaneCredentials,
	presented: string,
): Promise<PersonalAccessTokenSession | null> {
	try {
		return await sessionForPersonalAccessToken(credentials, presented);
	} catch (refusal) {
		if (refusal instanceof TokenOwnerHasLeft) error(403, refusal.message);
		throw refusal;
	}
}

export async function callingMember(request: Request, environment: Environment): Promise<CallingMember> {
	const projectURL = environment.SUPABASE_URL ?? '';
	const publishableKey = environment.SUPABASE_PUBLISHABLE_KEY ?? '';
	const serviceRoleKey = environment.SUPABASE_SECRET_KEY ?? environment.SUPABASE_SERVICE_ROLE_KEY ?? '';
	if (!projectURL || !publishableKey || !serviceRoleKey) {
		error(500, 'the control plane is not configured');
	}

	const { accessToken, permission, tokenName } = await memberAccessTokenOf(request, {
		projectURL,
		serviceRoleKey,
	});

	const caller = asMember({ projectURL, publishableKey }, accessToken);
	const { data: account } = await caller.auth.getUser();
	if (!account.user) error(401, 'sign in first');

	const member = await caller
		.from('member')
		.select('id, company_id, email')
		.eq('user_id', account.user.id)
		.maybeSingle<{ id: string; company_id: string; email: string | null }>();
	if (member.error) error(500, member.error.message);
	if (!member.data) error(403, 'refused');

	return {
		accessToken,
		caller,
		record: controlPlane({ projectURL, serviceRoleKey }),
		memberID: member.data.id,
		companyID: member.data.company_id,
		email: member.data.email ?? account.user.email ?? '',
		permission,
		tokenName
	};
}
