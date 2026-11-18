import { belongsInPhotoGallery, hasPhotoGallerySaving, saveIntoPhotoGallery } from '$lib/native-shell/media-save';
import { hasNativeShareSheet, presentShareSheet } from '$lib/native-shell/share-sheet';
import { isInsideTauriPhoneShell } from '$lib/native-shell/shell';

export function keepsFilesOnThePhone(): boolean {
	return isInsideTauriPhoneShell() && hasNativeShareSheet();
}

export async function keepOnThePhone(file: Blob, filename: string): Promise<void> {
	if (hasPhotoGallerySaving() && belongsInPhotoGallery(file.type)) {
		await saveIntoPhotoGallery(file, filename);
		return;
	}
	await presentShareSheet(file, filename);
}
