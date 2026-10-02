import type { z } from 'zod';
import { categoryName, type DataRoomCategory } from './model';
import type { companyDocumentResultSchema } from './schemas';
import type { FileBrowserEntry } from '$lib/components/file-browser-list.svelte';

export type DataRoomDocument = z.infer<typeof companyDocumentResultSchema>;

export function documentsInCategory(
	documents: DataRoomDocument[],
	categories: DataRoomCategory[],
	code: string
): DataRoomDocument[] {
	if (!code) return documents;
	const categoryCodes = new Set(
		categories
			.filter((category) => category.code === code || category.parent === code)
			.map((category) => category.code)
	);
	return documents.filter(
		(document) => document.categoryCode !== null && categoryCodes.has(document.categoryCode)
	);
}

export function documentCategoryLabel(
	document: DataRoomDocument,
	categories: DataRoomCategory[],
	locale: string
): string {
	const category = categories.find((candidate) => candidate.code === document.categoryCode);
	if (category) return categoryName(category, locale);
	return locale === 'ko' ? '기존 자료' : 'Legacy';
}

export function documentFileName(document: DataRoomDocument): string {
	const storedFileName = document.storagePath?.split('/').pop();
	if (storedFileName && !/^[a-f0-9]{64}$/.test(storedFileName)) return storedFileName;
	return document.filePath?.split('/').pop() || document.title;
}

export function dataRoomFolderEntries(
	documents: DataRoomDocument[],
	categories: DataRoomCategory[],
	code: string,
	locale: string
): FileBrowserEntry[] {
	const folders = categories.filter((category) => category.parent === (code || null));
	const folderEntries = folders.map((category) => ({
		id: category.code,
		name: categoryName(category, locale),
		isDirectory: true,
		secondary: category.code,
		date: ''
	}));
	const fileEntries = documents
		.filter((document) => document.categoryCode === code)
		.map((document) => ({
			id: document.documentID,
			name: documentFileName(document),
			fileName: documentFileName(document),
			secondary: document.categoryCode ?? '',
			date: document.date ?? ''
		}));
	return [...folderEntries, ...fileEntries];
}
