import type { SupabaseClient } from '@supabase/supabase-js';
import { assetBucket } from '$lib/messenger/kept-attachment';

const readableForSeconds = 60 * 60 * 24;

export async function pictureURLOfMember(client: SupabaseClient, memberID: string): Promise<string> {
	if (!memberID) return '';

	const member = await client
		.from('member')
		.select('profile_image')
		.eq('id', memberID)
		.maybeSingle<{ profile_image: string | null }>();
	if (member.error) throw new Error(member.error.message);

	const path = member.data?.profile_image ?? '';
	if (!path) return '';

	const signed = await client.storage.from(assetBucket).createSignedUrl(path, readableForSeconds);
	if (signed.error) return '';
	return signed.data?.signedUrl ?? '';
}
