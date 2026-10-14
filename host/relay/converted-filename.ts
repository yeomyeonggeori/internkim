const extensionOfConvertedType: Record<string, string> = {
	'video/mp4': 'mp4'
};

export function filenameAfterConversion(filename: string, offeredType: string, keptType: string): string {
	if (keptType === offeredType) return filename;
	const extension = extensionOfConvertedType[keptType];
	if (!extension) return filename;
	const base = filename.replace(/\.[^.]+$/, '');
	return `${base || 'media'}.${extension}`;
}
