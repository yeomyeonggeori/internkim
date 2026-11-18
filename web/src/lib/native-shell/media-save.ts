import { sendFileInChunks } from './file-chunks';

export const mediaSavePresenceMarker = '__INTERNKIM_MEDIA_SAVE__';
const vectorPictureType = 'image/svg+xml';

export function hasPhotoGallerySaving(): boolean {
	return typeof window !== 'undefined' && mediaSavePresenceMarker in window;
}

export function belongsInPhotoGallery(mediaType: string): boolean {
	if (mediaType === vectorPictureType) return false;
	return mediaType.startsWith('image/') || mediaType.startsWith('video/');
}

export async function saveIntoPhotoGallery(file: Blob, filename: string): Promise<void> {
	await sendFileInChunks('plugin:media-save|save', file, filename);
}
