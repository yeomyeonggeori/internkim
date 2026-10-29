const companyPictureFormatNames: Record<string, string> = {
	'image/png': 'PNG',
	'image/jpeg': 'JPG',
	'image/gif': 'GIF',
	'image/webp': 'WebP',
	'image/svg+xml': 'SVG',
	'image/heic': 'HEIC',
	'image/heif': 'HEIF'
};

export const companyPictureFormats = Object.keys(companyPictureFormatNames);

export const companyPictureFormatsAsNamed = Object.values(companyPictureFormatNames).join(', ');

export const companyPictureMegabytes = 10;

export const largestPictureACompanyCanHave = companyPictureMegabytes * 1024 * 1024;

export const companyPicturePath = '/company/profile-image';

export const companyPictureImage = 'picture';

export function refusalOfCompanyPictureSize(sizeBytes: number): string {
	if (sizeBytes <= largestPictureACompanyCanHave) return '';
	return `${(sizeBytes / 1024 / 1024).toFixed(1)}MB`;
}

export function isCompanyPictureFormat(contentType: string): boolean {
	return companyPictureFormats.includes(contentType.split(';')[0].trim().toLowerCase());
}
