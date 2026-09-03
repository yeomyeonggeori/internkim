export const companyPictureFormats = [
	'image/png',
	'image/jpeg',
	'image/gif',
	'image/webp',
	'image/heic',
	'image/heif'
];

export const companyPictureMegabytes = 10;

export const largestPictureACompanyCanHave = companyPictureMegabytes * 1024 * 1024;

export const companyPicturePath = '/company/profile-image';

export function refusalOfCompanyPictureSize(sizeBytes: number): string {
	if (sizeBytes <= largestPictureACompanyCanHave) return '';
	return `${(sizeBytes / 1024 / 1024).toFixed(1)}MB`;
}

export function isCompanyPictureFormat(contentType: string): boolean {
	return companyPictureFormats.includes(contentType.split(';')[0].trim().toLowerCase());
}
