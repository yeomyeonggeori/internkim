const extensionPattern = /^[a-z0-9]{1,10}$/;
const stemLength = 60;
const unnamedExtension = 'bin';

export type FiledDocument = { companyID: string; categoryCode: string; documentID: string };

export function dataRoomFolder(companyID: string, categoryCode: string): string {
	const parent = categoryCode.slice(0, 1);
	const folder = `${companyID}/dataroom/${parent}`;
	return categoryCode.length > 1 ? `${folder}/${categoryCode}` : folder;
}

export function originalPath(document: FiledDocument, fileName: string, fallbackNames: string[]): string {
	const extensionAt = fileName.lastIndexOf('.');
	const extension = extensionAt > 0 ? fileName.slice(extensionAt + 1).toLowerCase() : '';
	const stem = [extensionAt > 0 ? fileName.slice(0, extensionAt) : fileName, ...fallbackNames]
		.map(slugOf)
		.find(Boolean);
	const folder = dataRoomFolder(document.companyID, document.categoryCode);
	return `${folder}/${stem ?? 'document'}.${document.documentID}.${extensionPattern.test(extension) ? extension : unnamedExtension}`;
}

export function derivedPath(original: string, documentID: string, fileName: string): string {
	const marker = `.${documentID}.`;
	return original.slice(0, original.lastIndexOf(marker) + marker.length) + fileName;
}

function slugOf(value: string): string {
	return value
		.normalize('NFKD')
		.replace(/[̀-ͯ]/g, '')
		.replace(/[^A-Za-z0-9]+/g, '-')
		.replace(/^-+|-+$/g, '')
		.slice(0, stemLength)
		.replace(/-+$/, '')
		.toLowerCase();
}
