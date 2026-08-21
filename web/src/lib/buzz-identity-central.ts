import { generateSecretKey, getPublicKey } from 'nostr-tools/pure';
import type { SupabaseClient } from '@supabase/supabase-js';
import type {
	BuzzClaim,
	BuzzIdentityTransport,
	BuzzVaultDocument,
	BuzzVaultLookup
} from './buzz-identity-session';
import { bytesToHex, type WrappedSecret } from './buzz-key-vault';

const VAULT_KIND_PREFIX = 'buzz-vault-';

export function centralBuzzIdentityTransport(
	client: SupabaseClient,
	accountID: string
): BuzzIdentityTransport {
	return {
		// A device asks admind for its key over Cloudflare Access. A company has
		// nothing to ask, and nothing should be handed a secret it could open.
		async claim(): Promise<BuzzClaim> {
			const secret = generateSecretKey();
			return { secretHex: bytesToHex(secret), publicHex: getPublicKey(secret) };
		},
		async fetchVault(): Promise<BuzzVaultLookup> {
			const memberID = await memberIDOf(client, accountID);
			const { data, error } = await client
				.from('credential')
				.select('kind, settings')
				.eq('member_id', memberID)
				.like('kind', `${VAULT_KIND_PREFIX}%`);
			if (error) throw new Error(error.message);
			const copies = (data ?? [])
				.map((row) => (row as { settings: WrappedSecret | null }).settings)
				.filter((copy): copy is WrappedSecret => copy !== null);
			if (copies.length === 0) return { found: false };
			return { found: true, document: { copies } };
		},
		async storeVault(document: BuzzVaultDocument): Promise<void> {
			const memberID = await memberIDOf(client, accountID);
			const { error } = await client.from('credential').upsert(
				document.copies.map((copy) => ({
					member_id: memberID,
					company_id: null,
					kind: VAULT_KIND_PREFIX + copy.kind,
					external_id: null,
					// write_member_secret would put this in Supabase Vault, which the
					// server can decrypt. Whoever holds a Buzz secret key is that
					// person, so the sealed copy stays ciphertext in a plain column.
					vault_secret_id: null,
					settings: copy
				})),
				{ onConflict: 'member_id,kind' }
			);
			if (error) throw new Error(error.message);
		}
	};
}

async function memberIDOf(client: SupabaseClient, accountID: string): Promise<string> {
	const { data, error } = await client
		.from('member')
		.select('id')
		.eq('user_id', accountID)
		.single<{ id: string }>();
	if (error) throw new Error(error.message);
	return data.id;
}
