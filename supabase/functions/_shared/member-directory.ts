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

export type NotificationSenderFields = { senderID: string; senderName: string; icon: string };

type MemberWhoSends = { id: string; name: string | null; company_id: string | null; profile_image: string | null };

export async function colleagueWhoSent(
	client: SupabaseClient,
	senderID: string,
	recipientID: string
): Promise<NotificationSenderFields | null> {
	if (!senderID || !recipientID) return null;

	const { data, error } = await client
		.from('member')
		.select('id, name, company_id, profile_image')
		.in('id', [senderID, recipientID])
		.returns<MemberWhoSends[]>();
	if (error) throw new Error(error.message);

	const sender = data.find((member) => member.id === senderID);
	const recipient = data.find((member) => member.id === recipientID);
	if (!sender?.company_id || sender.company_id !== recipient?.company_id) return null;

	return {
		senderID: sender.id,
		senderName: (sender.name ?? '').trim(),
		icon: await signedPictureURL(client, sender.profile_image ?? '')
	};
}

export async function signedPictureURL(client: SupabaseClient, path: string): Promise<string> {
	if (!path) return '';
	const signed = await client.storage.from(assetBucket).createSignedUrl(path, readableForSeconds);
	if (signed.error) return '';
	return signed.data?.signedUrl ?? '';
}
