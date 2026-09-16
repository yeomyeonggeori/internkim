export const memberPicturePath = '/member/profile-image';

export const drawnPictureFormat = 'image/png';

export const drawnPictureSize = 256;

export const keptPictureFormats = ['image/png', 'image/jpeg', 'image/webp', 'image/gif', 'image/heic', 'image/heif'];

export const largestKeptPictureBytes = 5 * 1024 * 1024;

export function isKeptPictureFormat(contentType: string): boolean {
	return keptPictureFormats.includes(contentType.split(';')[0].trim().toLowerCase());
}

export function acceptedMemberPicture(picture: Blob): Blob {
	if (!isKeptPictureFormat(picture.type)) {
		throw new Error(`the messenger picture is ${picture.type}, which a member picture cannot be`);
	}
	return picture;
}
