//   monkeys run @production bun run web/scripts/issue-record-signing-key.ts

import { exportJWK, generateKeyPair } from 'jose';
import { rememberSetting } from './repository-setting';

async function privateJWK(): Promise<Record<string, unknown>> {
	const { privateKey } = await generateKeyPair('ES256', { extractable: true });
	const exported = await exportJWK(privateKey);
	return { ...exported, kid: crypto.randomUUID(), use: 'sig', alg: 'ES256', key_ops: ['sign', 'verify'], ext: true };
}

const key = await privateJWK();
rememberSetting('SUPABASE_JWT_SIGNING_KEY', JSON.stringify(key));

console.log(`signing key ${key.kid} is remembered as SUPABASE_JWT_SIGNING_KEY.`);
console.log('Import it in the dashboard: Project Settings > JWT Keys > Create standby key > Import an existing private key, and paste:');
console.log(JSON.stringify(key));
console.log('Then carry it to Pages: monkeys run @production bun run web/scripts/set-pages-secrets.ts --project internkim');
