import type { SupabaseClient } from './service-client.ts';

const assetBucket = 'asset';
const readableForSeconds = 60 * 60 * 24;

export async function membersOfCompanyByExternalID(
	client: SupabaseClient,
	companyID: string,
	platform: string
): Promise<Map<string, string>> {
	const { data, error } = await client
		.from('member')
		.select('id, messenger')
		.eq('company_id', companyID)
		.returns<{ id: string; messenger: Record<string, string> | null }[]>();
	if (error) throw new Error(error.message);
	return new Map(
		data
			.map((member) => [member.messenger?.[platform] ?? '', member.id] as const)
			.filter(([externalID]) => externalID !== '')
	);
}

export async function pictureURLOfMember(client: SupabaseClient, memberID: string): Promise<string> {
	if (!memberID) return '';

	const member = await client
		.from('member')
		.select('profile_image')
		.eq('id', memberID)
		.maybeSingle<{ profile_image: string | null }>();
	if (member.error) throw new Error(member.error.message);

	return signedPictureURL(client, member.data?.profile_image ?? '');
}

export async function signedPictureURL(client: SupabaseClient, path: string): Promise<string> {
	if (!path) return '';
	const signed = await client.storage.from(assetBucket).createSignedUrl(path, readableForSeconds);
	if (signed.error) return '';
	return signed.data?.signedUrl ?? '';
}
