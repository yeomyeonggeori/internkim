import type { z } from 'zod';
import { matchesKoreanSearch } from '$lib/korean-search';
import type { sharedDataRoomSchema } from './schemas';

export type SharedRoom = z.infer<typeof sharedDataRoomSchema>;
export type SharedDocument = SharedRoom['documents'][number];
export type SharedCategory = SharedRoom['categories'][number];
export type GuestFolder = { code: string; name: string; isChild: boolean; documentCount: number };

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

export function guestFolders(room: SharedRoom, locale: string): GuestFolder[] {
	const folderOf = (category: SharedCategory): GuestFolder => ({
		code: category.code,
		name: sharedCategoryName(category, locale),
		isChild: category.parent !== null,
		documentCount: documentsInFolder(room, category.code).length
	});
	return room.categories
		.filter((category) => category.parent === null)
		.flatMap((parent) => [
			folderOf(parent),
			...room.categories.filter((category) => category.parent === parent.code).map(folderOf)
		])
		.filter((folder) => folder.documentCount > 0);
}

export function matchingDocuments(documents: SharedDocument[], query: string): SharedDocument[] {
	return documents.filter((document) =>
		matchesKoreanSearch(`${document.title} ${document.summary ?? ''}`, query)
	);
}
