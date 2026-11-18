import { sendFileInChunks } from './file-chunks';

const vectorPictureType = 'image/svg+xml';

export function belongsInPhotoGallery(mediaType: string): boolean {
	if (mediaType === vectorPictureType) return false;
	return mediaType.startsWith('image/') || mediaType.startsWith('video/');
}

export async function saveIntoPhotoGallery(file: Blob, filename: string): Promise<void> {
	await sendFileInChunks('plugin:media-save|save', file, filename);
}
