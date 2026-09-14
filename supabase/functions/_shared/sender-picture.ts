import { pictureURLOfMember, signedPictureURL } from './member-directory.ts';
import type { SupabaseClient } from './service-client.ts';

export async function pictureURLOfSender(
	client: SupabaseClient,
	companyID: string,
	keptPath: unknown,
	memberID: string
): Promise<string> {
	if (isCompanySharedPath(companyID, keptPath)) return signedPictureURL(client, keptPath);
	return pictureURLOfMember(client, memberID);
}

export function isCompanySharedPath(companyID: string, path: unknown): path is string {
	if (typeof path !== 'string' || !companyID) return false;
	const segments = path.split('/');
	if (segments[0] !== companyID || segments[1] !== 'shared' || segments.length < 3) return false;
	return segments.every((segment) => segment !== '' && segment !== '.' && segment !== '..');
}
