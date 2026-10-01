import type { SupabaseClient } from '@supabase/supabase-js';
import { decodeJwt, errors, importJWK, jwtVerify } from 'jose';
import { z } from 'zod';
import { companyComputerName } from '$lib/company/host-setup';
import {
	boxConfigurationSchema,
	boxKeySchema,
	sealedSecretSchema,
	type BoxConfiguration,
	type ConnectedBox,
	type EmptyBox,
	type SealedSecret
} from '$lib/company/box';
import type { Environment } from './agent-request';
import {
	claimFleetForCompany,
	companyOfFleet,
	controlPlane,
	fleetCredentialKind,
	hashOf,
	hostSessionOfCompany,
	spendAgentKey,
	type HostSession,
	type SigningCredentials
} from './control-plane';

export const boxAssertionAudience = 'internkim-box';
const boxAssertionLifetime = '2m';
const emptyBoxFreshMilliseconds = 10 * 60 * 1000;
const emptyBoxesListed = 10;
const pairingCodeAlphabet = 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789';
const pairingCodeLength = 8;
const pairingCodeLifetimeMilliseconds = 15 * 60 * 1000;

export class BoxRefused extends Error {
	constructor(
		message: string,
		readonly status: 409 | 429 = 409
	) {
		super(message);
	}
}

export type BoxAnnounced =
	| { isClaimed: true }
	| { isClaimed: false; pairingCode?: string; pairingCodeExpiresAt?: string };

type ClaimOutcome = 'claimed' | 'wrong_code' | 'no_live_code' | 'too_many_attempts';

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

export type BoxAnnouncement = {
	publicKey: string;
	encryptionKey: string;
	publicAddress: string;
	wantsPairingCode: boolean;
	hostName?: string;
	pairingPageAddresses?: string[];
};

export async function announceBox(
	client: SupabaseClient,
	announcement: BoxAnnouncement,
	now: Date = new Date()
): Promise<BoxAnnounced> {
	const { publicKey } = announcement;
	if (await companyOfFleet(client, publicKey)) return { isClaimed: true };
	const { error } = await client.from('empty_box').upsert(
		{
			public_key: publicKey,
			encryption_key: announcement.encryptionKey,
			public_address: announcement.publicAddress,
			host_name: announcement.hostName ?? null,
			pairing_page_addresses: announcement.pairingPageAddresses ?? [],
			announced_at: now.toISOString()
		},
		{ onConflict: 'public_key' }
	);
	if (error) throw new Error(`announcing box ${publicKey}: ${error.message}`);
	if (!announcement.wantsPairingCode && (await holdsLivePairingCode(client, publicKey, now))) {
		return { isClaimed: false };
	}
	return { isClaimed: false, ...(await issuePairingCode(client, publicKey, now)) };
}

async function holdsLivePairingCode(client: SupabaseClient, publicKey: string, now: Date): Promise<boolean> {
	const { data, error } = await client
		.from('empty_box')
		.select('pairing_code_expires_at')
		.eq('public_key', publicKey)
		.single<{ pairing_code_expires_at: string | null }>();
	if (error) throw new Error(`pairing code of box ${publicKey}: ${error.message}`);
	return data.pairing_code_expires_at !== null && Date.parse(data.pairing_code_expires_at) > now.getTime();
}

async function issuePairingCode(
	client: SupabaseClient,
	publicKey: string,
	now: Date
): Promise<{ pairingCode: string; pairingCodeExpiresAt: string }> {
	const pairingCode = freshPairingCode();
	const pairingCodeExpiresAt = new Date(now.getTime() + pairingCodeLifetimeMilliseconds).toISOString();
	const { error } = await client
		.from('empty_box')
		.update({
			pairing_code_hash: await hashOf(normalizedPairingCode(pairingCode)),
			pairing_code_expires_at: pairingCodeExpiresAt,
			wrong_pairing_codes: 0
		})
		.eq('public_key', publicKey);
	if (error) throw new Error(`pairing code of box ${publicKey}: ${error.message}`);
	return { pairingCode, pairingCodeExpiresAt };
}

function freshPairingCode(): string {
	const symbols = [...crypto.getRandomValues(new Uint8Array(pairingCodeLength))].map(
		(byte) => pairingCodeAlphabet[byte % pairingCodeAlphabet.length]
	);
	return `${symbols.slice(0, 4).join('')}-${symbols.slice(4).join('')}`;
}

function normalizedPairingCode(typed: string): string {
	return typed.toUpperCase().replace(/[^0-9A-Z]/g, '');
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
		.select('public_key, announced_at, host_name, pairing_page_addresses')
		.eq('public_address', publicAddress)
		.gte('announced_at', freshSince(now))
		.order('announced_at', { ascending: false })
		.limit(emptyBoxesListed);
	if (error) throw new Error(`empty boxes at ${publicAddress}: ${error.message}`);
	return data.map((row) => ({
		publicKey: row.public_key,
		announcedAt: row.announced_at,
		hostName: row.host_name,
		pairingPageAddresses: row.pairing_page_addresses
	}));
}

export async function claimBox(
	client: SupabaseClient,
	companyID: string,
	publicKey: string,
	pairingCode: string,
	now: Date = new Date()
): Promise<void> {
	const { data, error } = await client
		.rpc('claim_empty_box', {
			claiming_company: companyID,
			box_key: publicKey,
			code_hash: await hashOf(normalizedPairingCode(pairingCode)),
			fresh_since: freshSince(now)
		})
		.single<{ outcome: ClaimOutcome; encryption_key: string | null }>();
	if (error) throw new Error(`claiming box ${publicKey}: ${error.message}`);
	if (data.outcome === 'too_many_attempts') {
		throw new BoxRefused('too many wrong codes from this company; try again in an hour', 429);
	}
	if (data.outcome === 'no_live_code') {
		throw new BoxRefused('that box shows no code right now; wait a minute and use the one it shows next');
	}
	if (data.outcome === 'wrong_code' || !data.encryption_key) {
		throw new BoxRefused('that is not the code the box shows');
	}
	await claimFleetForCompany(client, companyID, publicKey, { encryptionKey: data.encryption_key });
}

export async function claimBoxWithConnectionFile(
	client: SupabaseClient,
	publicKey: string,
	encryptionKey: string,
	connectionKey: string
): Promise<string | null> {
	const companyID = await spendAgentKey(client, connectionKey, companyComputerName);
	if (!companyID) return null;
	await claimFleetForCompany(client, companyID, publicKey, { encryptionKey });
	const removed = await client.from('empty_box').delete().eq('public_key', publicKey);
	if (removed.error) throw new Error(`claiming box ${publicKey}: ${removed.error.message}`);
	return companyID;
}

const legacySealedModelKeySchema = z.object({
	ephemeralPublicKey: boxKeySchema,
	nonce: z.string().regex(/^[A-Za-z0-9_-]{16}$/),
	ciphertext: z.string().regex(/^[A-Za-z0-9_-]+$/).max(4096)
}).strict();

const heldModelKeySchema = z.union([sealedSecretSchema, legacySealedModelKeySchema]);

type HeldModelKey = z.infer<typeof heldModelKeySchema>;

const boxSettingsSchema = z.object({
	encryptionKey: boxKeySchema,
	lastSeenAt: z.string().optional(),
	sealedModelKey: heldModelKeySchema.optional()
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
		companyID: box.companyID,
		publicKey: box.publicKey,
		encryptionKey: box.settings.encryptionKey,
		lastSeenAt: box.settings.lastSeenAt ?? null,
		hasModelKey: Boolean(box.settings.sealedModelKey)
	};
}

export async function keepSealedModelKey(
	client: SupabaseClient,
	companyID: string,
	sealedModelKey: SealedSecret
): Promise<void> {
	const box = await boxOfCompany(client, companyID);
	if (!box) throw new BoxRefused('connect the company computer before giving it a model key');
	await writeBoxSettings(client, box, { ...box.settings, sealedModelKey });
}

export type BoxSession = {
	configuration: BoxConfiguration;
	session: HostSession;
	sealedModelKey: HeldModelKey | null;
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
