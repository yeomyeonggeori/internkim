import { fileToAttachment } from '$lib/components/channel/channel-attachments';
import { pictureFileEveryBrowserShows } from '$lib/files/heic-picture';
import { keepAttachmentForSending, setOwnMessengerPicture } from '$lib/messenger/messenger-api';
import { personPicture } from '$lib/stores/person-picture.svelte';

const formatsEveryBrowserShows = ['image/png', 'image/jpeg', 'image/webp'];

export const ownPictureFormats = [...formatsEveryBrowserShows, 'image/heic', 'image/heif'];

export class PictureFormatRefused extends Error {
	constructor(format: string) {
		super(`a profile picture is one of ${ownPictureFormats.join(', ')}, and this one is ${format || 'of no known format'}`);
		this.name = 'PictureFormatRefused';
	}
}

export async function changeOwnPicture(chosen: File): Promise<void> {
	const shown = await pictureFileEveryBrowserShows(chosen);
	if (!formatsEveryBrowserShows.includes(shown.type)) throw new PictureFormatRefused(shown.type);
	const kept = await keepAttachmentForSending(await fileToAttachment(shown));
	const known = await setOwnMessengerPicture(kept.address);
	await personPicture.showChangedPicture(known.externalID, known.avatarURL);
}
