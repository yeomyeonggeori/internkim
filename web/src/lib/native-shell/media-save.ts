import { sendFileInChunks, type ReportSentBytes } from './file-chunks';
import type { NativeDownload } from './file-download';

export const mediaSavePresenceMarker = '__INTERNKIM_MEDIA_SAVE__';
export const photoPermissionRefusalReason = 'photoPermission';
const vectorPictureType = 'image/svg+xml';

export function hasPhotoGallerySaving(): boolean {
	return typeof window !== 'undefined' && mediaSavePresenceMarker in window;
}

export function belongsInPhotoGallery(mediaType: string): boolean {
	if (mediaType === vectorPictureType) return false;
	return mediaType.startsWith('image/') || mediaType.startsWith('video/');
}

export function wasRefusedPhotoPermission(failure: unknown): boolean {
	return typeof failure === 'object' && failure !== null && 'reason' in failure && failure.reason === photoPermissionRefusalReason;
}

export async function saveIntoPhotoGallery(file: Blob, filename: string, onSent: ReportSentBytes): Promise<void> {
	await sendFileInChunks('plugin:media-save|save', file, filename, onSent);
}

export async function saveDownloadIntoPhotoGallery(download: NativeDownload): Promise<void> {
	const { invoke } = await import('@tauri-apps/api/core');
	await invoke<void>('plugin:media-save|save_download', { transfer: download.transfer, mediaType: download.mediaType });
}
