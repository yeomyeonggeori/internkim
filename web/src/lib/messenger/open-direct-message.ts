import { goto } from '$app/navigation';
import { ensureDirectMessage } from '$lib/components/channel/channel-api';
import { companyPathOf, companySlugOf } from '$lib/company-path';
import { externalIDFor, fetchMessengerDirectory } from './messenger-directory';

export type DirectMessageOpening = 'opened' | 'no-messenger-account';

export async function openDirectMessageWith(person: { memberID: string; email: string }, pathname: string): Promise<DirectMessageOpening> {
	const externalID = externalIDFor(person, await fetchMessengerDirectory());
	if (!externalID) return 'no-messenger-account';
	const channelID = await ensureDirectMessage(externalID);
	await goto(`${companyPathOf(companySlugOf(pathname), '/messenger')}?channel=${encodeURIComponent(channelID)}`);
	return 'opened';
}
