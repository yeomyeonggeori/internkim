import { toast } from 'svelte-sonner';
import { createPageText } from '$lib/i18n/page-text.svelte';
import { phoneDownloadText } from '$lib/i18n/phone-download-text';
import { belongsInPhotoGallery, hasPhotoGallerySaving, saveIntoPhotoGallery } from '$lib/native-shell/media-save';
import { hasNativeShareSheet, presentShareSheet } from '$lib/native-shell/share-sheet';
import { isInsideTauriPhoneShell } from '$lib/native-shell/shell';

const text = createPageText(phoneDownloadText);

export function keepsFilesOnThePhone(): boolean {
	return isInsideTauriPhoneShell() && hasNativeShareSheet();
}

async function saveIntoGalleryAndTell(file: Blob, filename: string): Promise<void> {
	try {
		await saveIntoPhotoGallery(file, filename);
	} catch (failure) {
		toast.error(text.notSavedToGallery);
		throw failure;
	}
	toast.success(text.savedToGallery);
}

async function presentShareSheetOrTell(file: Blob, filename: string): Promise<void> {
	try {
		await presentShareSheet(file, filename);
	} catch (failure) {
		toast.error(text.shareSheetFailed);
		throw failure;
	}
}

export async function keepOnThePhone(file: Blob, filename: string): Promise<void> {
	if (hasPhotoGallerySaving() && belongsInPhotoGallery(file.type)) {
		await saveIntoGalleryAndTell(file, filename);
		return;
	}
	await presentShareSheetOrTell(file, filename);
}
