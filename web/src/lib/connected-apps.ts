import type { OAuthAuthorizationDetails } from '@supabase/supabase-js';
import { hostOf } from '$lib/consent-return';
import { supabase } from '$lib/supabase';

export type ConsentRequest =
	| { kind: 'asking'; details: OAuthAuthorizationDetails }
	| { kind: 'decided'; redirectURL: string }
	| { kind: 'unreadable' };

export async function consentRequestOf(authorizationID: string): Promise<ConsentRequest> {
	const { data, error } = await supabase().auth.oauth.getAuthorizationDetails(authorizationID);
	if (error) return { kind: 'unreadable' };
	if ('authorization_id' in data) return { kind: 'asking', details: data };
	return { kind: 'decided', redirectURL: data.redirect_url };
}

export async function answerConsent(authorizationID: string, isApproved: boolean): Promise<string> {
	const oauth = supabase().auth.oauth;
	const options = { skipBrowserRedirect: true };
	const { data, error } = isApproved
		? await oauth.approveAuthorization(authorizationID, options)
		: await oauth.denyAuthorization(authorizationID, options);
	if (error) throw new Error(error.message);
	return data.redirect_url;
}

export type ConnectedApp = { clientID: string; name: string; grantedAt: string };

export async function connectedApps(): Promise<ConnectedApp[]> {
	const { data, error } = await supabase().auth.oauth.listGrants();
	if (error) throw new Error(error.message);
	return data.map((grant) => ({
		clientID: grant.client.id,
		name: grant.client.name || hostOf(grant.client.uri),
		grantedAt: grant.granted_at
	}));
}

export async function disconnectApp(clientID: string): Promise<void> {
	const { error } = await supabase().auth.oauth.revokeGrant({ clientId: clientID });
	if (error) throw new Error(error.message);
}
