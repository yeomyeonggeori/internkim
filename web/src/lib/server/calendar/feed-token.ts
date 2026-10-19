import type { SupabaseClient } from '@supabase/supabase-js';
import { controlPlane, type ControlPlaneCredentials } from '$lib/server/control-plane';
import { RecordRefusedTheWrite, statusOfPostgresCode } from '$lib/server/public-api/record/tasks';

async function hashOf(secret: string): Promise<string> {
	const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(secret));
	return [...new Uint8Array(digest)].map((byte) => byte.toString(16).padStart(2, '0')).join('');
}

function mintedToken(): string {
	const bytes = crypto.getRandomValues(new Uint8Array(32));
	return [...bytes].map((byte) => byte.toString(16).padStart(2, '0')).join('');
}

export function feedTokenOfPath(written: string): string {
	const asked = written.trim().toLowerCase();
	return /^[0-9a-f]{64}$/.test(asked) ? asked : '';
}

export async function companyOfFeedToken(
	credentials: ControlPlaneCredentials,
	token: string
): Promise<string | null> {
	const asked = feedTokenOfPath(token);
	if (!asked) return null;

	const { data, error } = await controlPlane(credentials).rpc('calendar_subscription_company', {
		target_token_hash: await hashOf(asked)
	});
	if (error) throw new Error(`calendar subscription: ${error.message}`);
	return typeof data === 'string' && data ? data : null;
}

export function feedAddressOf(appAddress: string, token: string): string {
	return `${appAddress}/calendar/feed/${token}.ics`;
}

export async function issueFeedToken(caller: SupabaseClient): Promise<string> {
	const token = mintedToken();
	await saveFeedTokenHash(caller, await hashOf(token));
	return token;
}

export async function forgetFeedToken(caller: SupabaseClient): Promise<void> {
	await saveFeedTokenHash(caller, null);
}

async function saveFeedTokenHash(caller: SupabaseClient, tokenHash: string | null): Promise<void> {
	const { error } = await caller.rpc('calendar_subscription_save', { target_token_hash: tokenHash });
	if (error) throw new RecordRefusedTheWrite(`calendar subscription: ${error.message}`, statusOfPostgresCode(error.code));
}

export async function companyHasAFeedToken(caller: SupabaseClient, companyID: string): Promise<boolean> {
	const { data, error } = await caller
		.from('company')
		.select('calendar')
		.eq('id', companyID)
		.maybeSingle<{ calendar: { subscription?: { tokenHash?: string } } }>();
	if (error) throw new Error(`calendar subscription: ${error.message}`);
	return Boolean(data?.calendar?.subscription?.tokenHash);
}
