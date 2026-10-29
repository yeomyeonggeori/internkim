import type { z } from 'zod';
import type { FileBrowserEntry } from '$lib/components/file-browser-list.svelte';
import { matchesKoreanSearch } from '$lib/korean-search';
import type { sharedDataRoomSchema } from './schemas';

export type SharedRoom = z.infer<typeof sharedDataRoomSchema>;
export type SharedDocument = SharedRoom['documents'][number];
export type SharedCategory = SharedRoom['categories'][number];

export function sharedCategoryName(category: SharedCategory, locale: string): string {
	return locale === 'ko' ? category.name_ko || category.name : category.name;
}

export function sharedFileName(document: SharedDocument): string | null {
	return document.extension ? `${document.title}.${document.extension}` : null;
}

export function documentsInFolder(room: SharedRoom, code: string): SharedDocument[] {
	if (!code) return room.documents;
	const codes = new Set(
		room.categories
			.filter((category) => category.code === code || category.parent === code)
			.map((category) => category.code)
	);
	return room.documents.filter((document) => codes.has(document.category_code));
}

export function subfolderEntries(room: SharedRoom, code: string, locale: string): FileBrowserEntry[] {
	return room.categories
		.filter((category) => category.parent === (code || null))
		.map((category) => ({
			id: category.code,
			name: sharedCategoryName(category, locale),
			isDirectory: true,
			entryCount: documentsInFolder(room, category.code).length,
			cells: []
		}))
		.filter((folder) => folder.entryCount > 0);
}

export function listedDocuments(room: SharedRoom, code: string, query: string): SharedDocument[] {
	if (query) return matchingDocuments(documentsInFolder(room, code), query);
	return room.documents.filter((document) => document.category_code === code);
}

export function matchingDocuments(documents: SharedDocument[], query: string): SharedDocument[] {
	return documents.filter((document) =>
		matchesKoreanSearch(`${document.title} ${document.summary ?? ''}`, query)
	);
}
