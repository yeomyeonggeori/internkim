//   bun run web/scripts/issue-record-signing-key.ts [--status standby|in_use]
//
// Mints the elliptic curve key the web app signs record tokens with, registers
// it with the Supabase project as a JWT signing key so PostgREST, Storage and
// Realtime verify those tokens, and keeps the private half in the repository
// root settings file as SUPABASE_JWT_SIGNING_KEY for set-pages-secrets.ts to
// carry to Pages.

import { exportJWK, generateKeyPair } from 'jose';
import { accessToken, projectReference } from './remote-query';
import { keepSetting, repositorySettingsPath } from './repository-setting';

type KeyStatus = 'standby' | 'in_use';

function argument(name: string): string | undefined {
	const index = process.argv.indexOf(`--${name}`);
	return index >= 0 ? process.argv[index + 1] : undefined;
}

function statusOf(written: string | undefined): KeyStatus {
	if (written === undefined || written === 'standby') return 'standby';
	if (written === 'in_use') return 'in_use';
	throw new Error(`--status takes standby or in_use, not ${written}`);
}

async function privateJWK(): Promise<Record<string, unknown>> {
	const { privateKey } = await generateKeyPair('ES256', { extractable: true });
	const exported = await exportJWK(privateKey);
	return { ...exported, kid: crypto.randomUUID(), use: 'sig', alg: 'ES256', key_ops: ['sign', 'verify'], ext: true };
}

async function registerWithSupabase(key: Record<string, unknown>, status: KeyStatus): Promise<{ id: string }> {
	const response = await fetch(`https://api.supabase.com/v1/projects/${projectReference}/config/auth/signing-keys`, {
		method: 'POST',
		headers: { Authorization: `Bearer ${accessToken()}`, 'Content-Type': 'application/json' },
		body: JSON.stringify({ algorithm: 'ES256', status, private_jwk: key })
	});
	const body = await response.text();
	if (!response.ok) throw new Error(`Supabase refused the signing key: ${response.status} ${body}`);
	return JSON.parse(body) as { id: string };
}

const status = statusOf(argument('status'));
const key = await privateJWK();
const registered = await registerWithSupabase(key, status);
keepSetting('SUPABASE_JWT_SIGNING_KEY', JSON.stringify(key));

console.log(`signing key ${registered.id} (kid ${key.kid}) is ${status} on project ${projectReference}`);
console.log(`its private half is SUPABASE_JWT_SIGNING_KEY in ${repositorySettingsPath}`);
console.log('carry it to Pages with: bun run web/scripts/set-pages-secrets.ts --project internkim');
