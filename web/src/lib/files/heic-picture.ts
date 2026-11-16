const heicFormats = ['image/heic', 'image/heif'];
const heicFileName = /\.(heic|heif)$/i;
const shownFormat = 'image/jpeg';
const shownQuality = 0.9;

export function isHeicPicture(picture: Blob): boolean {
	const format = picture.type.split(';')[0].trim().toLowerCase();
	if (format) return heicFormats.includes(format);
	return picture instanceof File && heicFileName.test(picture.name);
}

export async function pictureEveryBrowserShows(picture: Blob): Promise<Blob> {
	if (!isHeicPicture(picture)) return picture;
	const { heicTo } = await import('heic-to/csp');
	try {
		return await heicTo({ blob: picture, type: shownFormat, quality: shownQuality });
	} catch (failure) {
		const reason = failure instanceof Error ? failure.message : String(failure);
		throw new Error(`this HEIC picture could not be turned into a JPEG (${reason}); save it as JPEG or PNG and choose it again`, {
			cause: failure
		});
	}
}

export async function pictureFileEveryBrowserShows(file: File): Promise<File> {
	const picture = await pictureEveryBrowserShows(file);
	if (picture === file) return file;
	return new File([picture], file.name.replace(heicFileName, '.jpg'), { type: picture.type });
}
