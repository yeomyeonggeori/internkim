import type { SupabaseClient } from '@supabase/supabase-js';
import { decodeJwt, errors, importJWK, jwtVerify } from 'jose';
import { z } from 'zod';
import {
	boxConfigurationSchema,
	boxKeySchema,
	sealedModelKeySchema,
	type BoxConfiguration,
	type ConnectedBox,
	type EmptyBox,
	type SealedModelKey
} from '$lib/company/box';
import type { Environment } from './agent-request';
import {
	claimFleetForCompany,
	companyOfFleet,
	controlPlane,
	fleetCredentialKind,
	hostSessionOfCompany,
	type HostSession,
	type SigningCredentials
} from './control-plane';

export const boxAssertionAudience = 'internkim-box';
const boxAssertionLifetime = '2m';
const emptyBoxFreshMilliseconds = 10 * 60 * 1000;
const emptyBoxesListed = 10;

export class BoxRefused extends Error {}

export async function boxKeyOfAssertion(assertion: string): Promise<string | null> {
	const publicKey = claimedBoxKeyOf(assertion);
	if (!publicKey) return null;
	const verifyingKey = await importJWK({ kty: 'OKP', crv: 'Ed25519', x: publicKey }, 'EdDSA');
	try {
		await jwtVerify(assertion, verifyingKey, {
			issuer: publicKey,
			audience: boxAssertionAudience,
			algorithms: ['EdDSA'],
			maxTokenAge: boxAssertionLifetime,
			requiredClaims: ['iat', 'exp']
		});
		return publicKey;
	} catch (failure) {
		if (failure instanceof errors.JOSEError) return null;
		throw failure;
	}
}

function claimedBoxKeyOf(assertion: string): string | null {
	try {
		const claimed = boxKeySchema.safeParse(decodeJwt(assertion).iss);
		return claimed.success ? claimed.data : null;
	} catch (failure) {
		if (failure instanceof errors.JOSEError) return null;
		throw failure;
	}
}

export async function announceBox(
	client: SupabaseClient,
	publicKey: string,
	encryptionKey: string,
	publicAddress: string
): Promise<{ isClaimed: boolean }> {
	if (await companyOfFleet(client, publicKey)) return { isClaimed: true };
	const { error } = await client.from('empty_box').upsert(
		{
			public_key: publicKey,
			encryption_key: encryptionKey,
			public_address: publicAddress,
			announced_at: new Date().toISOString()
		},
		{ onConflict: 'public_key' }
	);
	if (error) throw new Error(`announcing box ${publicKey}: ${error.message}`);
	return { isClaimed: false };
}

function freshSince(now: Date): string {
	return new Date(now.getTime() - emptyBoxFreshMilliseconds).toISOString();
}

export async function emptyBoxesAt(
	client: SupabaseClient,
	publicAddress: string,
	now: Date = new Date()
): Promise<EmptyBox[]> {
	const { data, error } = await client
		.from('empty_box')
		.select('public_key, announced_at')
		.eq('public_address', publicAddress)
		.gte('announced_at', freshSince(now))
		.order('announced_at', { ascending: false })
		.limit(emptyBoxesListed);
	if (error) throw new Error(`empty boxes at ${publicAddress}: ${error.message}`);
	return data.map((row) => ({ publicKey: row.public_key, announcedAt: row.announced_at }));
}

export async function claimBox(
	client: SupabaseClient,
	companyID: string,
	publicKey: string,
	publicAddress: string,
	now: Date = new Date()
): Promise<void> {
	const { data, error } = await client
		.from('empty_box')
		.select('encryption_key')
		.eq('public_key', publicKey)
		.eq('public_address', publicAddress)
		.gte('announced_at', freshSince(now))
		.maybeSingle<{ encryption_key: string }>();
	if (error) throw new Error(`claiming box ${publicKey}: ${error.message}`);
	if (!data) throw new BoxRefused('that box is not announcing itself from this network');

	await claimFleetForCompany(client, companyID, publicKey, { encryptionKey: data.encryption_key });
	const removed = await client.from('empty_box').delete().eq('public_key', publicKey);
	if (removed.error) throw new Error(`claiming box ${publicKey}: ${removed.error.message}`);
}

const boxSettingsSchema = z.object({
	encryptionKey: boxKeySchema,
	lastSeenAt: z.string().optional(),
	sealedModelKey: sealedModelKeySchema.optional()
});

type BoxSettings = z.infer<typeof boxSettingsSchema>;

type BoxRow = { companyID: string; publicKey: string; settings: BoxSettings };

async function boxOfCompany(client: SupabaseClient, companyID: string): Promise<BoxRow | null> {
	const { data, error } = await client
		.from('credential')
		.select('external_id, settings')
		.eq('company_id', companyID)
		.eq('kind', fleetCredentialKind)
		.maybeSingle<{ external_id: string; settings: unknown }>();
	if (error) throw new Error(`box of ${companyID}: ${error.message}`);
	const settings = boxSettingsSchema.safeParse(data?.settings);
	if (!data || !settings.success) return null;
	return { companyID, publicKey: data.external_id, settings: settings.data };
}

async function writeBoxSettings(client: SupabaseClient, box: BoxRow, settings: BoxSettings): Promise<void> {
	const { error } = await client
		.from('credential')
		.update({ settings })
		.eq('company_id', box.companyID)
		.eq('kind', fleetCredentialKind)
		.eq('external_id', box.publicKey);
	if (error) throw new Error(`box of ${box.companyID}: ${error.message}`);
}

export async function connectedBoxOf(client: SupabaseClient, companyID: string): Promise<ConnectedBox | null> {
	const box = await boxOfCompany(client, companyID);
	if (!box) return null;
	return {
		publicKey: box.publicKey,
		encryptionKey: box.settings.encryptionKey,
		lastSeenAt: box.settings.lastSeenAt ?? null,
		hasModelKey: Boolean(box.settings.sealedModelKey)
	};
}

export async function keepSealedModelKey(
	client: SupabaseClient,
	companyID: string,
	sealedModelKey: SealedModelKey
): Promise<void> {
	const box = await boxOfCompany(client, companyID);
	if (!box) throw new BoxRefused('connect the company computer before giving it a model key');
	await writeBoxSettings(client, box, { ...box.settings, sealedModelKey });
}

export type BoxSession = {
	configuration: BoxConfiguration;
	session: HostSession;
	sealedModelKey: SealedModelKey | null;
};

export async function boxSessionFor(
	credentials: SigningCredentials,
	publicKey: string,
	environment: Environment,
	appURL: string
): Promise<BoxSession | null> {
	const client = controlPlane(credentials);
	const companyID = await companyOfFleet(client, publicKey);
	if (!companyID) return null;
	const box = await boxOfCompany(client, companyID);
	if (!box) return null;

	await writeBoxSettings(client, box, { ...box.settings, lastSeenAt: new Date().toISOString() });
	return {
		configuration: await boxConfigurationOf(client, companyID, environment, appURL),
		session: await hostSessionOfCompany(credentials, companyID),
		sealedModelKey: box.settings.sealedModelKey ?? null
	};
}

async function boxConfigurationOf(
	client: SupabaseClient,
	companyID: string,
	environment: Environment,
	appURL: string
): Promise<BoxConfiguration> {
	const { data, error } = await client
		.from('company')
		.select('id, name, slug')
		.eq('id', companyID)
		.single<{ id: string; name: string; slug: string }>();
	if (error) throw new Error(`company ${companyID}: ${error.message}`);
	const configuration = boxConfigurationSchema.safeParse({
		schemaVersion: 1,
		appURL,
		company: data,
		centralPlane: { projectURL: environment.SUPABASE_URL, publishableKey: environment.SUPABASE_PUBLISHABLE_KEY },
		gatewayURL: environment.GATEWAY_URL
	});
	if (!configuration.success) throw new Error('the company computer connection is not configured');
	return configuration.data;
}
