import type { PreviewSource } from '$lib/files/preview/document-preview.svelte';
import { previewKindOf } from '$lib/files/preview/kind';

export type ViewerDocument = {
	id: string;
	title: string;
	fileName: string | null;
	category: string;
	date: string;
	summary: string;
	details: [label: string, value: string][];
};

export type ViewerSource = PreviewSource & { isTextPreview: boolean };

export type FileSigner = (derivedFileName?: string) => Promise<string>;

export const textPreviewFileName = 'content.txt';

export class DataRoomRefused extends Error {
	constructor(
		message: string,
		readonly status: number
	) {
		super(message);
		this.name = 'DataRoomRefused';
	}
}

export function isAccessRefusal(error: unknown): boolean {
	if (!(error instanceof Error) || !('status' in error)) return false;
	return error.status === 400 || error.status === 403 || error.status === 404;
}

export async function previewSourceOf(
	fileName: string,
	signURL: FileSigner,
	mayReadOriginal: boolean
): Promise<ViewerSource | null> {
	if (mayReadOriginal && previewKindOf(fileName) !== 'none') {
		const originalURL = await refusedAsNull(signURL());
		if (originalURL) return { url: originalURL, fileName, isTextPreview: false };
	}
	const textURL = await refusedAsNull(signURL(textPreviewFileName));
	return textURL ? { url: textURL, fileName: textPreviewFileName, isTextPreview: true } : null;
}

async function refusedAsNull(request: Promise<string>): Promise<string | null> {
	return request.catch((error: unknown) => {
		if (isAccessRefusal(error)) return null;
		throw error;
	});
}

export function neighbourID(
	documents: ViewerDocument[],
	openID: string,
	step: -1 | 1
): string | undefined {
	const index = documents.findIndex((document) => document.id === openID);
	if (index < 0) return undefined;
	return documents[index + step]?.id;
}
