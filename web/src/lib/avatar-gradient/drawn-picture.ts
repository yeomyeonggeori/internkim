import { personAvatarSeed } from '$lib/person-avatar-seed';
import { drawnPictureFormat, drawnPictureSize } from '$lib/profile/member-picture';
import { renderGradient, toSeed } from './engine';

export async function drawnPictureOf(email: string): Promise<Blob> {
	const canvas = new OffscreenCanvas(drawnPictureSize, drawnPictureSize);
	renderGradient(canvas, toSeed(personAvatarSeed(email, '', '')), {
		pattern: 'mesh',
		displaySize: drawnPictureSize
	});
	return canvas.convertToBlob({ type: drawnPictureFormat });
}
