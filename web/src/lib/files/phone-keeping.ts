import type { ReportSentBytes } from '$lib/native-shell/file-chunks';
import type { NativeDownload } from '$lib/native-shell/file-download';
import {
	belongsInPhotoGallery,
	hasPhotoGallerySaving,
	saveDownloadIntoPhotoGallery,
	saveIntoPhotoGallery
} from '$lib/native-shell/media-save';
import { hasNativeShareSheet, presentDownloadInShareSheet, presentShareSheet } from '$lib/native-shell/share-sheet';
import { isInsideTauriPhoneShell } from '$lib/native-shell/shell';
import type { KeptFile } from './kept-file';

export function keepsFilesOnThePhone(): boolean {
	return isInsideTauriPhoneShell() && hasNativeShareSheet();
}

function belongsInTheGallery(mediaType: string): boolean {
	return hasPhotoGallerySaving() && belongsInPhotoGallery(mediaType);
}

export async function keepOnThePhone(file: Blob, filename: string, onSent: ReportSentBytes): Promise<KeptFile> {
	if (belongsInTheGallery(file.type)) {
		await saveIntoPhotoGallery(file, filename, onSent);
		return 'photoLibrary';
	}
	await presentShareSheet(file, filename, onSent);
	return 'shareSheet';
}

export async function keepDownloadOnThePhone(download: NativeDownload): Promise<KeptFile> {
	if (belongsInTheGallery(download.mediaType)) {
		await saveDownloadIntoPhotoGallery(download);
		return 'photoLibrary';
	}
	await presentDownloadInShareSheet(download);
	return 'shareSheet';
}
