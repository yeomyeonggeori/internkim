import {
	JSONWebKeyCache,
	TokenRefused,
	resolveMember,
	verifyToken,
	type FetchDocument
} from '../../connection-gateway/src/identity';
import type { Caller } from './personal-access-token';

export type SessionEnvironment = {
	SUPABASE_URL: string;
	SUPABASE_SECRET_KEY: string;
};

const sessionPermission = 'delete';

export function issuerOf(supabaseURL: string): string {
	return `${supabaseURL.replace(/\/+$/, '')}/auth/v1`;
}

export function jwksURLOf(supabaseURL: string): string {
	return `${issuerOf(supabaseURL)}/.well-known/jwks.json`;
}

export async function callerOfSessionToken(
	environment: SessionEnvironment,
	keyCache: JSONWebKeyCache,
	presented: string,
	nowSeconds: number,
	fetchDocument?: FetchDocument
): Promise<Caller> {
	const claims = await verifyToken(presented, keyCache, issuerOf(environment.SUPABASE_URL), nowSeconds);
	const identity = await resolveMember(
		environment.SUPABASE_URL,
		environment.SUPABASE_SECRET_KEY,
		presented,
		claims.sub,
		fetchDocument
	);
	if (!identity.email) throw new TokenRefused('this account carries no email');
	return {
		email: identity.email,
		companyID: identity.companyID,
		memberID: identity.memberID,
		tokenName: '',
		permission: sessionPermission
	};
}
