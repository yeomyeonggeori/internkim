import { adoptRememberedMessengerScope, messengerCacheKey } from '$lib/messenger/cache-scope';
import { freshSignedURLOf, readKeptAccounts, readKeptPictures, type KeptAccounts, type KeptPicture } from './person-picture-cache';
import { keptPictureFiles } from './person-picture-files';

export type RestoredPictures = {
	kept: Map<string, KeptPicture>;
	accounts: KeptAccounts | null;
	shownOfExternal: Map<string, string>;
};

export async function restoreKeptPictures(): Promise<RestoredPictures | null> {
	const scope = messengerCacheKey() || (await adoptRememberedMessengerScope())?.key || '';
	if (!scope) return null;
	const kept = readKeptPictures(scope);
	const files = await keptPictureFiles([...kept.values()].map((picture) => picture.address));
	const shownOfExternal = new Map<string, string>();
	for (const [externalID, picture] of kept) {
		const shown = files.get(picture.address) || freshSignedURLOf(picture);
		if (shown) shownOfExternal.set(externalID, shown);
	}
	return { kept, accounts: readKeptAccounts(scope), shownOfExternal };
}
