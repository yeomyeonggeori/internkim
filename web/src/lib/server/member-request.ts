import { error } from '@sveltejs/kit';
import { decodeJwt, errors } from 'jose';
import type { SupabaseClient } from '@supabase/supabase-js';
import { fullPublicAPIPermission, reachesPermission, type PublicAPIPermission } from '$lib/public-api-permission';
import {
	asMember,
	controlPlane,
	isPersonalAccessToken,
	planeCredentialsOf,
	sessionForPersonalAccessToken,
	TokenOwnerHasLeft,
	type PersonalAccessTokenSession,
	type SigningCredentials,
} from './control-plane';
import { theAppAddressOf } from './company-host-redirect';
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
	memberID: string | null;
};

export async function memberAccessTokenOf(
	request: Request,
	credentials: SigningCredentials,
): Promise<MemberCall> {
	const authorization = request.headers.get('authorization') ?? '';
	const presented = authorization.startsWith('Bearer ') ? authorization.slice('Bearer '.length) : '';
	if (!presented) error(401, 'sign in first');
	if (!isPersonalAccessToken(presented)) {
		return { accessToken: presented, permission: fullPublicAPIPermission, tokenName: '', memberID: null };
	}

	const session = await sessionOfTokenOrRefusal(credentials, presented);
	if (!session) error(401, 'that key belongs to nobody');
	return {
		accessToken: session.accessToken,
		permission: session.permission,
		tokenName: session.tokenName,
		memberID: session.memberID,
	};
}

export async function signedInAccessTokenOf(request: Request, credentials: SigningCredentials): Promise<string> {
	const { accessToken, tokenName } = await memberAccessTokenOf(request, credentials);
	if (tokenName) error(403, 'sign in to administer the company; a personal access token does not');
	if (isGrantedToAnOAuthClient(accessToken)) {
		error(403, 'sign in to administer the company; a token granted to another application does not');
	}
	return accessToken;
}

function isGrantedToAnOAuthClient(accessToken: string): boolean {
	try {
		return typeof decodeJwt(accessToken).client_id === 'string';
	} catch (failure) {
		if (failure instanceof errors.JWTInvalid) return false;
		throw failure;
	}
}

export function refuseUnlessTheCallerWrites(member: Pick<CallingMember, 'permission'>): void {
	if (!reachesPermission(member.permission, 'write')) error(403, 'this token may not write');
}

async function sessionOfTokenOrRefusal(
	credentials: SigningCredentials,
	presented: string,
): Promise<PersonalAccessTokenSession | null> {
	try {
		return await sessionForPersonalAccessToken(credentials, presented);
	} catch (refusal) {
		if (refusal instanceof TokenOwnerHasLeft) error(403, refusal.message);
		throw refusal;
	}
}

type MemberRow = { id: string; company_id: string; email: string | null };

async function memberByID(caller: SupabaseClient, memberID: string): Promise<MemberRow | null> {
	const member = await caller
		.from('member')
		.select('id, company_id, email')
		.eq('id', memberID)
		.maybeSingle<MemberRow>();
	if (member.error) error(500, member.error.message);
	return member.data;
}

async function memberOfSignedInAccount(caller: SupabaseClient): Promise<MemberRow | null> {
	const { data: account } = await caller.auth.getUser();
	if (!account.user) error(401, 'sign in first');
	const member = await caller
		.from('member')
		.select('id, company_id, email')
		.eq('user_id', account.user.id)
		.maybeSingle<MemberRow>();
	if (member.error) error(500, member.error.message);
	if (!member.data) return null;
	return { ...member.data, email: member.data.email ?? account.user.email ?? null };
}

export async function callingMember(request: Request, environment: Environment): Promise<CallingMember> {
	const plane = planeCredentialsOf(environment);
	if (!plane) error(500, 'the control plane is not configured');

	const { accessToken, permission, tokenName, memberID } = await memberAccessTokenOf(request, plane);
	const caller = asMember(plane, accessToken);
	const member = memberID ? await memberByID(caller, memberID) : await memberOfSignedInAccount(caller);
	if (!member && memberID) error(403, 'the member this key was issued to is gone');
	if (!member) error(403, `this account belongs to no company yet; start one at ${theAppAddressOf(environment)}/start`);

	return {
		accessToken,
		caller,
		record: controlPlane(plane),
		memberID: member.id,
		companyID: member.company_id,
		email: member.email ?? '',
		permission,
		tokenName
	};
}
