import type { SupabaseClient } from '@supabase/supabase-js';
import { decodeJwt, errors, importJWK, jwtVerify } from 'jose';
import { z } from 'zod';
import { companyComputerName } from '$lib/company/host-setup';
import { base64URLOf } from '$lib/company/seal-to-box';
import {
	adminPasswordOutcomeSchema,
	boxConfigurationSchema,
	boxKeySchema,
	boxRoleSchema,
	keptBackupManifestSchema,
	nearbyNetworkListSchema,
	sealedSecretSchema,
	wifiOutcomeSchema,
	type AwaitingBox,
	type BoxConfiguration,
	type BoxPlacement,
	type BoxRole,
	type TakeOver,
	type CompanyBoxes,
	type ConnectedBox,
	type EmptyBox,
	type KeptBackupManifest,
	type SealedSecret
} from '$lib/company/box';
import type { Environment } from './agent-request';
import {
	controlPlane,
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
const boxColumns = 'public_key, company_id, encryption_key, role, host_name, site, weight, settings, sealed_company_key, last_seen_at';

export class BoxRefused extends Error {
	constructor(
		message: string,
		readonly status: 400 | 404 | 409 | 429 = 409
	) {
		super(message);
	}
}

export type BoxAnnounced =
	| { isClaimed: true }
	| { isClaimed: false; pairingCode?: string; pairingCodeExpiresAt?: string };

type VerificationOutcome = 'verified' | 'claimed' | 'wrong_code' | 'no_live_code' | 'no_live_ticket' | 'too_many_attempts';

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
	if (await boxOfPublicKey(client, publicKey)) return { isClaimed: true };
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

export type VerifiedBox = {
	ticket: string;
	publicKey: string;
	hostName: string | null;
	publicAddress: string;
};

export async function verifyBoxCode(
	client: SupabaseClient,
	companyID: string,
	publicKey: string,
	pairingCode: string,
	now: Date = new Date()
): Promise<VerifiedBox> {
	const ticket = base64URLOf(crypto.getRandomValues(new Uint8Array(32)));
	const { data, error } = await client
		.rpc('verify_box_pairing_code', {
			claiming_company: companyID,
			box_key: publicKey,
			code_hash: await hashOf(normalizedPairingCode(pairingCode)),
			ticket_hash: await claimTicketHashOf(ticket),
			fresh_since: freshSince(now)
		})
		.single<{ outcome: VerificationOutcome; host_name: string | null; public_address: string | null }>();
	if (error) throw new Error(`verifying box ${publicKey}: ${error.message}`);
	refuseUnless(data.outcome, 'verified');
	return { ticket, publicKey, hostName: data.host_name, publicAddress: data.public_address ?? '' };
}

export type ClaimIntent = 'add' | 'replace';

export async function claimBox(
	client: SupabaseClient,
	companyID: string,
	publicKey: string,
	ticket: string,
	intent: ClaimIntent,
	now: Date = new Date()
): Promise<BoxRole> {
	const { data, error } = await client
		.rpc('claim_empty_box', {
			claiming_company: companyID,
			box_key: publicKey,
			ticket_hash: await claimTicketHashOf(ticket),
			fresh_since: freshSince(now),
			is_replacing: intent === 'replace'
		})
		.single<{ outcome: VerificationOutcome; given_role: string | null }>();
	if (error) throw new Error(`claiming box ${publicKey}: ${error.message}`);
	refuseUnless(data.outcome, 'claimed');
	return boxRoleSchema.parse(data.given_role);
}

async function keepClaimedBox(
	client: SupabaseClient,
	companyID: string,
	publicKey: string,
	encryptionKey: string,
	hostName: string | null,
	intent: ClaimIntent
): Promise<BoxRole> {
	const { data, error } = await client.rpc('keep_claimed_box', {
		claiming_company: companyID,
		box_key: publicKey,
		box_encryption_key: encryptionKey,
		box_host_name: hostName,
		is_replacing: intent === 'replace'
	});
	if (error) throw new Error(`keeping box ${publicKey} for ${companyID}: ${error.message}`);
	return boxRoleSchema.parse(data);
}

function claimTicketHashOf(ticket: string): Promise<string> {
	return hashOf(`claim-ticket:${ticket}`);
}

const refusalOfOutcome: Record<Exclude<VerificationOutcome, 'verified' | 'claimed'>, [string, 409 | 429]> = {
	too_many_attempts: ['too many wrong codes from this company; try again in an hour', 429],
	no_live_code: ['that box shows no code right now; wait a minute and use the one it shows next', 409],
	wrong_code: ['that is not the code the box shows', 409],
	no_live_ticket: ['that confirmation has expired; enter the code the box shows now', 409]
};

function refuseUnless(outcome: VerificationOutcome, expected: 'verified' | 'claimed'): void {
	if (outcome === expected) return;
	if (outcome === 'verified' || outcome === 'claimed') throw new Error(`the record answered ${outcome}`);
	throw new BoxRefused(...refusalOfOutcome[outcome]);
}

export async function releaseBox(client: SupabaseClient, companyID: string, publicKey: string): Promise<boolean> {
	const { data, error } = await client
		.from('box')
		.delete()
		.eq('company_id', companyID)
		.eq('public_key', publicKey)
		.select('public_key');
	if (error) throw new Error(`releasing box ${publicKey} of ${companyID}: ${error.message}`);
	return data.length > 0;
}

export async function releaseBoxOfKey(client: SupabaseClient, publicKey: string): Promise<boolean> {
	const box = await boxOfPublicKey(client, publicKey);
	if (!box) return false;
	return releaseBox(client, box.companyID, publicKey);
}

export async function claimBoxWithConnectionFile(
	client: SupabaseClient,
	publicKey: string,
	encryptionKey: string,
	connectionKey: string
): Promise<string | null> {
	const companyID = await spendAgentKey(client, connectionKey, companyComputerName);
	if (!companyID) return null;
	await keepClaimedBox(client, companyID, publicKey, encryptionKey, null, 'replace');
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

const wifiChangeSchema = z.object({
	requestID: z.uuid(),
	sealed: sealedSecretSchema,
	requestedAt: z.iso.datetime({ offset: true })
}).strict();

const adminPasswordSettingSchema = z.object({
	settingID: z.uuid(),
	sealed: sealedSecretSchema,
	requestedAt: z.iso.datetime({ offset: true })
}).strict();

const nearbyNetworksSettingSchema = z.object({
	networks: nearbyNetworkListSchema,
	scannedAt: z.iso.datetime({ offset: true })
}).strict();

const boxSettingsSchema = z.object({
	wifiChange: wifiChangeSchema.optional(),
	wifiOutcome: wifiOutcomeSchema.optional(),
	nearbyNetworks: nearbyNetworksSettingSchema.optional(),
	hasAdminAccount: z.literal(true).optional(),
	adminPassword: adminPasswordSettingSchema.optional(),
	adminPasswordOutcome: adminPasswordOutcomeSchema.optional()
});

type BoxSettings = z.infer<typeof boxSettingsSchema>;

const boxRecordSchema = z.object({
	public_key: boxKeySchema,
	company_id: z.string(),
	encryption_key: boxKeySchema,
	role: boxRoleSchema,
	host_name: z.string().nullable(),
	site: z.string(),
	weight: z.number().int(),
	settings: boxSettingsSchema,
	sealed_company_key: sealedSecretSchema.nullable(),
	last_seen_at: z.string().nullable()
});

type BoxRecord = z.infer<typeof boxRecordSchema>;

export type BoxRow = {
	companyID: string;
	publicKey: string;
	encryptionKey: string;
	role: BoxRole;
	hostName: string | null;
	site: string;
	weight: number;
	settings: BoxSettings;
	sealedCompanyKey: SealedSecret | null;
	lastSeenAt: string | null;
};

function boxRowOf(record: BoxRecord): BoxRow {
	return {
		companyID: record.company_id,
		publicKey: record.public_key,
		encryptionKey: record.encryption_key,
		role: record.role,
		hostName: record.host_name,
		site: record.site,
		weight: record.weight,
		settings: record.settings,
		sealedCompanyKey: record.sealed_company_key,
		lastSeenAt: record.last_seen_at
	};
}

export async function boxOfPublicKey(client: SupabaseClient, publicKey: string): Promise<BoxRow | null> {
	const { data, error } = await client.from('box').select(boxColumns).eq('public_key', publicKey).maybeSingle();
	if (error) throw new Error(`box ${publicKey}: ${error.message}`);
	if (!data) return null;
	return boxRowOf(boxRecordSchema.parse(data));
}

export async function boxOfCompany(client: SupabaseClient, companyID: string, publicKey: string): Promise<BoxRow | null> {
	const box = await boxOfPublicKey(client, publicKey);
	return box?.companyID === companyID ? box : null;
}

export async function boxesOfCompany(client: SupabaseClient, companyID: string): Promise<BoxRow[]> {
	const { data, error } = await client
		.from('box')
		.select(boxColumns)
		.eq('company_id', companyID)
		.order('claimed_at', { ascending: true });
	if (error) throw new Error(`boxes of ${companyID}: ${error.message}`);
	return data.map((record) => boxRowOf(boxRecordSchema.parse(record)));
}

export async function writeBoxSettings(client: SupabaseClient, box: BoxRow, settings: BoxSettings): Promise<void> {
	const { error } = await client
		.from('box')
		.update({ settings })
		.eq('company_id', box.companyID)
		.eq('public_key', box.publicKey);
	if (error) throw new Error(`box ${box.publicKey} of ${box.companyID}: ${error.message}`);
}

const keyringRecordSchema = z.object({
	sealing_key: boxKeySchema,
	sealed_model_key: heldModelKeySchema.nullable()
});

export type Keyring = { sealingKey: string; sealedModelKey: HeldModelKey | null };

export async function keyringOf(client: SupabaseClient, companyID: string): Promise<Keyring | null> {
	const { data, error } = await client
		.from('box_keyring')
		.select('sealing_key, sealed_model_key')
		.eq('company_id', companyID)
		.maybeSingle();
	if (error) throw new Error(`keyring of ${companyID}: ${error.message}`);
	if (!data) return null;
	const record = keyringRecordSchema.parse(data);
	return { sealingKey: record.sealing_key, sealedModelKey: record.sealed_model_key };
}

export function holdsCompanyKey(box: BoxRow, keyring: Keyring | null): boolean {
	if (!keyring) return false;
	return box.encryptionKey === keyring.sealingKey || box.sealedCompanyKey !== null;
}

export async function companyBoxesOf(client: SupabaseClient, companyID: string): Promise<CompanyBoxes> {
	const [boxes, keyring] = await Promise.all([boxesOfCompany(client, companyID), keyringOf(client, companyID)]);
	return {
		boxes: boxes.map((box) => connectedBoxOf(box, keyring)),
		sealingKey: keyring?.sealingKey ?? null,
		hasModelKey: Boolean(keyring?.sealedModelKey),
		newestBackupAt: keyring ? await newestKeptBackupAt(client, companyID, keyring.sealingKey) : null,
		takeOver: await takeOverOf(client, companyID)
	};
}

const keptBackupBucket = 'box_backup';

export async function newestKeptBackupAt(client: SupabaseClient, companyID: string, sealingKey: string): Promise<string | null> {
	const { data, error } = await client.storage
		.from(keptBackupBucket)
		.list(companyID, { limit: 1000, sortBy: { column: 'name', order: 'desc' } });
	if (error) throw new Error(`kept backups of ${companyID}: ${error.message}`);
	for (const archive of data) {
		const manifest = await keptManifestOf(client, `${companyID}/${archive.name}/manifest.json`);
		if (manifest?.archiveName === archive.name && manifest.sealingKey === sealingKey) return manifest.createdAt;
	}
	return null;
}

async function keptManifestOf(client: SupabaseClient, path: string): Promise<KeptBackupManifest | null> {
	const { data } = await client.storage.from(keptBackupBucket).download(path);
	if (!data) return null;
	const parsed = keptBackupManifestSchema.safeParse(JSON.parse(await data.text()));
	return parsed.success ? parsed.data : null;
}

const servingOutcomeSchema = z.enum(['serving', 'not_its_box', 'no_backup']);

export type ServingOutcome = z.infer<typeof servingOutcomeSchema>;

export async function makeBoxServe(client: SupabaseClient, companyID: string, publicKey: string): Promise<ServingOutcome> {
	const { data, error } = await client.rpc('make_box_serve', { serving_company: companyID, box_key: publicKey });
	if (error) throw new Error(`making box ${publicKey} serve ${companyID}: ${error.message}`);
	return servingOutcomeSchema.parse(data);
}

async function companyKeepsABackup(client: SupabaseClient, companyID: string): Promise<boolean> {
	const { data, error } = await client.rpc('company_keeps_a_backup', { kept_company: companyID });
	if (error) throw new Error(`kept backups of ${companyID}: ${error.message}`);
	return data === true;
}

function connectedBoxOf(box: BoxRow, keyring: Keyring | null): ConnectedBox {
	return {
		companyID: box.companyID,
		publicKey: box.publicKey,
		encryptionKey: box.encryptionKey,
		role: box.role,
		hostName: box.hostName,
		site: box.site,
		weight: box.weight,
		lastSeenAt: box.lastSeenAt,
		holdsCompanyKey: holdsCompanyKey(box, keyring),
		hasAdminAccount: box.settings.hasAdminAccount ?? false
	};
}

export async function keepSealedModelKey(
	client: SupabaseClient,
	companyID: string,
	sealedModelKey: SealedSecret
): Promise<void> {
	const keyring = await keyringOf(client, companyID);
	if (!keyring) throw new BoxRefused('connect the company computer before giving it a model key');
	if (sealedModelKey.recipient !== keyring.sealingKey) {
		throw new BoxRefused('the model key is sealed to a key the company no longer seals to; reload and give it again');
	}
	const { error } = await client
		.from('box_keyring')
		.update({ sealed_model_key: sealedModelKey })
		.eq('company_id', companyID)
		.eq('sealing_key', keyring.sealingKey);
	if (error) throw new Error(`model key of ${companyID}: ${error.message}`);
}

export async function keepSealedCompanyKeys(
	client: SupabaseClient,
	holder: BoxRow,
	sealedKeys: { publicKey: string; sealed: SealedSecret }[]
): Promise<void> {
	if (!holdsCompanyKey(holder, await keyringOf(client, holder.companyID))) {
		throw new BoxRefused('only a box that holds the company key may hand it on');
	}
	const awaiting = await boxesAwaitingCompanyKey(client, holder.companyID);
	for (const { publicKey, sealed } of sealedKeys) {
		const target = awaiting.find((box) => box.publicKey === publicKey);
		if (!target) throw new BoxRefused(`box ${publicKey} is not waiting for this company's key`, 404);
		if (sealed.recipient !== target.encryptionKey) throw new BoxRefused(`the key for ${publicKey} is sealed to another key`);
		const { error } = await client
			.from('box')
			.update({ sealed_company_key: sealed })
			.eq('company_id', holder.companyID)
			.eq('public_key', publicKey)
			.is('sealed_company_key', null);
		if (error) throw new Error(`company key for ${publicKey}: ${error.message}`);
	}
}

async function boxesAwaitingCompanyKey(client: SupabaseClient, companyID: string): Promise<AwaitingBox[]> {
	const [boxes, keyring] = await Promise.all([boxesOfCompany(client, companyID), keyringOf(client, companyID)]);
	if (!keyring) return [];
	return boxes
		.filter((box) => !holdsCompanyKey(box, keyring))
		.map((box) => ({ publicKey: box.publicKey, encryptionKey: box.encryptionKey }));
}

export type BoxSession = {
	configuration: BoxConfiguration;
	session: HostSession;
	role: BoxRole;
	sealingKey: string;
	sealedCompanyKey: SealedSecret | null;
	sealedModelKey: HeldModelKey | null;
	boxesAwaitingCompanyKey: AwaitingBox[];
	keepsBackups: boolean;
	takeOver: TakeOver;
};

export async function boxSessionFor(
	credentials: SigningCredentials,
	publicKey: string,
	environment: Environment,
	appURL: string
): Promise<BoxSession | null> {
	const client = controlPlane(credentials);
	const box = await boxOfPublicKey(client, publicKey);
	if (!box) return null;
	const keyring = await keyringOf(client, box.companyID);
	if (!keyring) throw new Error(`company ${box.companyID} has a box and no keyring`);

	const role = await noteBoxSeen(client, box);
	const holdsKey = holdsCompanyKey(box, keyring);
	return {
		configuration: await boxConfigurationOf(client, box.companyID, environment, appURL),
		session: await hostSessionOfCompany(credentials, box.companyID, { fleetID: publicKey }),
		role,
		sealingKey: keyring.sealingKey,
		sealedCompanyKey: box.sealedCompanyKey,
		sealedModelKey: holdsKey ? keyring.sealedModelKey : null,
		boxesAwaitingCompanyKey: holdsKey ? await boxesAwaitingCompanyKey(client, box.companyID) : [],
		keepsBackups: await companyKeepsABackup(client, box.companyID),
		takeOver: await takeOverOf(client, box.companyID)
	};
}

async function noteBoxSeen(client: SupabaseClient, box: BoxRow): Promise<BoxRole> {
	const { data, error } = await client.rpc('note_box_seen', { box_key: box.publicKey });
	if (error) throw new Error(`box ${box.publicKey}: ${error.message}`);
	return boxRoleSchema.parse(data);
}

const takeOverRecordSchema = z.object({ is_automatic: z.boolean(), lapse_seconds: z.number().int() });

export const defaultTakeOver: TakeOver = { isAutomatic: false, lapseSeconds: 300 };

export async function takeOverOf(client: SupabaseClient, companyID: string): Promise<TakeOver> {
	const { data, error } = await client
		.from('box_take_over')
		.select('is_automatic, lapse_seconds')
		.eq('company_id', companyID)
		.maybeSingle();
	if (error) throw new Error(`take-over of ${companyID}: ${error.message}`);
	if (!data) return defaultTakeOver;
	const record = takeOverRecordSchema.parse(data);
	return { isAutomatic: record.is_automatic, lapseSeconds: record.lapse_seconds };
}

const checkViolation = '23514';

export async function keepTakeOver(client: SupabaseClient, companyID: string, takeOver: TakeOver): Promise<void> {
	const { error } = await client
		.from('box_take_over')
		.upsert({ company_id: companyID, is_automatic: takeOver.isAutomatic, lapse_seconds: takeOver.lapseSeconds });
	if (error?.code === checkViolation) throw new BoxRefused(`the record refused this lapse: ${error.message}`, 400);
	if (error) throw new Error(`take-over of ${companyID}: ${error.message}`);
}

export async function placeBox(client: SupabaseClient, companyID: string, publicKey: string, placement: BoxPlacement): Promise<boolean> {
	const { data, error } = await client
		.from('box')
		.update({ site: placement.site, weight: placement.weight })
		.eq('company_id', companyID)
		.eq('public_key', publicKey)
		.select('public_key');
	if (error?.code === checkViolation) throw new BoxRefused(`the record refused this placement: ${error.message}`, 400);
	if (error) throw new Error(`placing box ${publicKey} of ${companyID}: ${error.message}`);
	return data.length === 1;
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
