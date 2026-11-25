import type { SupabaseClient } from '@supabase/supabase-js';
import { base64URLOf } from '../../src/lib/company/seal-to-box';
import { controlPlane, hostSessionOfCompany, type HostSession, type SigningCredentials } from '../../src/lib/server/control-plane';

export type ClaimedBox = { publicKey: string; encryptionKey: string };

export type ClaimedBoxWithSession = ClaimedBox & { session: HostSession };

function aBoxKey(): string {
	return base64URLOf(crypto.getRandomValues(new Uint8Array(32)));
}

export async function claimABox(client: SupabaseClient, companyID: string, encryptionKey: string = aBoxKey()): Promise<ClaimedBox> {
	const publicKey = aBoxKey();
	const { error } = await client.rpc('keep_claimed_box', {
		claiming_company: companyID,
		box_key: publicKey,
		box_encryption_key: encryptionKey,
		box_host_name: null,
		is_replacing: false
	});
	if (error) throw new Error(`claiming a box for ${companyID}: ${error.message}`);
	return { publicKey, encryptionKey };
}

export async function claimABoxWithItsSession(
	credentials: SigningCredentials,
	companyID: string,
	encryptionKey?: string
): Promise<ClaimedBoxWithSession> {
	const box = await claimABox(controlPlane(credentials), companyID, encryptionKey);
	return { ...box, session: await hostSessionOfCompany(credentials, companyID, { fleetID: box.publicKey }) };
}

export async function releaseTheClaimedBox(client: SupabaseClient, companyID: string, box: ClaimedBox): Promise<void> {
	const released = await client.from('box').delete().eq('public_key', box.publicKey);
	if (released.error) throw new Error(`releasing box ${box.publicKey}: ${released.error.message}`);
	const forgotten = await client.from('box_keyring').delete().eq('company_id', companyID).eq('sealing_key', box.encryptionKey);
	if (forgotten.error) throw new Error(`forgetting the keyring box ${box.publicKey} began: ${forgotten.error.message}`);
}
